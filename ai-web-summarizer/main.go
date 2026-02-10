package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"cloud.google.com/go/bigquery"
	"cloud.google.com/go/civil"
	"github.com/slack-go/slack"
	"google.golang.org/api/iterator"
	"google.golang.org/genai"

	"github.com/ureuzy/cloud_functions/ai-web-summarizer/config"
)

// BigQueryから取得するリリースノートの構造
type ReleaseNote struct {
	Description        string     `bigquery:"description"`
	ReleaseNoteType    string     `bigquery:"release_note_type"`
	PublishedAt        civil.Date `bigquery:"published_at"` // BigQueryのDATE型
	ProductName        string     `bigquery:"product_name"`
	ProductVersionName string     `bigquery:"product_version_name"`
}

type UpdateItem struct {
	Service string   `json:"service"`
	Title   string   `json:"title"`
	Date    string   `json:"date"`
	Summary []string `json:"summary"` // 3 points
	Link    string   `json:"link"`
}

type UpdateResponse struct {
	ServiceName string       `json:"service_name"`
	UpdatesDate string       `json:"updates_date"`
	Updates     []UpdateItem `json:"updates"`
}

func fetchReleaseNotesFromBigQuery(ctx context.Context, projectID string, targetDate string) ([]ReleaseNote, error) {
	client, err := bigquery.NewClient(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to create BigQuery client: %v", err)
	}
	defer client.Close()

	// targetDateは "2006-01-02" 形式で受け取る
	query := fmt.Sprintf(`
		SELECT
			description,
			release_note_type,
			published_at,
			product_name,
			COALESCE(product_version_name, '') as product_version_name
		FROM
			`+"`bigquery-public-data.google_cloud_release_notes.release_notes`"+`
		WHERE
			DATE(published_at) = '%s'
			AND release_note_type IN ('FEATURE', 'SERVICE_ANNOUNCEMENT', 'BREAKING_CHANGE', 'NON_BREAKING_CHANGE')
		ORDER BY release_note_type, published_at
	`, targetDate)

	q := client.Query(query)
	it, err := q.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %v", err)
	}

	var notes []ReleaseNote
	for {
		var note ReleaseNote
		err := it.Next(&note)
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to iterate results: %v", err)
		}
		notes = append(notes, note)
	}

	return notes, nil
}

func main() {
	ctx := context.Background()
	conf, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	slackClient := slack.New(conf.SlackBotToken)
	genaiClient, err := genai.NewClient(ctx, &genai.ClientConfig{
		Project:  conf.ProjectID,
		Location: conf.Location,
		Backend:  genai.BackendVertexAI,
	})
	if err != nil {
		log.Fatalf("Failed to create GenAI client: %v", err)
	}

	// リリースノートを取得する日付を計算
	jst := time.FixedZone("Asia/Tokyo", 9*60*60)
	targetDate := time.Now().In(jst).AddDate(0, 0, -conf.DaysAgo)
	targetDateStr := targetDate.Format("2006/01/02")        // 表示用（スラッシュ区切り）
	targetDateForQuery := targetDate.Format("2006-01-02") // BigQueryクエリ用（ハイフン区切り）

	log.Printf("--- Fetching Google Cloud Release Notes for %s ---", targetDateStr)

	// BigQueryからリリースノートを取得
	notes, err := fetchReleaseNotesFromBigQuery(ctx, conf.BigQueryProjectID, targetDateForQuery)
	if err != nil {
		log.Fatalf("Failed to fetch release notes from BigQuery: %v", err)
	}

	log.Printf("Found %d release notes", len(notes))

	if len(notes) == 0 {
		msg := fmt.Sprintf("[Google Cloud] %s のリリースノートはありません。", targetDateStr)
		_, _, err := slackClient.PostMessage(conf.Channel, slack.MsgOptionText(msg, false))
		if err != nil {
			log.Printf("Failed to send 'no updates' message: %v", err)
		}
		log.Println("No release notes found.")
		return
	}

	// ProductNameごとにグループ化
	notesByProduct := make(map[string][]ReleaseNote)
	for _, note := range notes {
		notesByProduct[note.ProductName] = append(notesByProduct[note.ProductName], note)
	}

	log.Printf("Grouped into %d products", len(notesByProduct))

	// 1. Google Cloud全体のヘッダー投稿を作成
	headerText := fmt.Sprintf("*Google Cloud Release Notes*\n%s のリリースノートは %d 件です（%d 製品）。", targetDateStr, len(notes), len(notesByProduct))
	blocks := []slack.Block{
		slack.NewSectionBlock(slack.NewTextBlockObject("mrkdwn", headerText, false, false), nil, nil),
		slack.NewContextBlock("", slack.NewTextBlockObject("mrkdwn", "<https://cloud.google.com/release-notes|Google Cloud Release Notes>", false, false)),
	}

	_, mainThreadTs, err := slackClient.PostMessage(conf.Channel,
		slack.MsgOptionBlocks(blocks...),
		slack.MsgOptionPostMessageParameters(slack.PostMessageParameters{
			UnfurlLinks: false,
			UnfurlMedia: false,
		}),
	)
	if err != nil {
		log.Fatalf("Failed to post main header: %v", err)
	}

	log.Printf("Posted main header with thread_ts: %s", mainThreadTs)

	// 2. 各製品ごとにスレッドに投稿
	processedCount := 0
	for productName, productNotes := range notesByProduct {
		log.Printf("Processing product: %s (%d notes)", productName, len(productNotes))

		// Geminiで要約
		updateResp, err := summarizeReleaseNotes(ctx, genaiClient, conf.ModelName, productNotes, targetDateStr, productName)
		if err != nil {
			log.Printf("Failed to summarize release notes for %s: %v", productName, err)
			continue
		}

		if len(updateResp.Updates) == 0 {
			log.Printf("No updates generated for %s", productName)
			continue
		}

		// 製品のヘッダーをスレッドに投稿（Blocksを使用）
		productHeaderText := fmt.Sprintf("📦 *%s*\n_%s のアップデート: %d 件_", productName, targetDateStr, len(updateResp.Updates))

		var productListItems []string
		for i, item := range updateResp.Updates {
			productListItems = append(productListItems, fmt.Sprintf("  %d. %s", i+1, item.Title))
		}

		productBlocks := []slack.Block{
			slack.NewDividerBlock(),
			slack.NewSectionBlock(
				slack.NewTextBlockObject("mrkdwn", productHeaderText, false, false),
				nil,
				nil,
			),
			slack.NewSectionBlock(
				slack.NewTextBlockObject("mrkdwn", strings.Join(productListItems, "\n"), false, false),
				nil,
				nil,
			),
		}

		_, _, err = slackClient.PostMessage(conf.Channel,
			slack.MsgOptionBlocks(productBlocks...),
			slack.MsgOptionTS(mainThreadTs),
			slack.MsgOptionPostMessageParameters(slack.PostMessageParameters{
				UnfurlLinks: false,
				UnfurlMedia: false,
			}),
		)
		if err != nil {
			log.Printf("Failed to post product header for %s: %v", productName, err)
			continue
		}

		// 詳細をスレッドに投稿
		for _, item := range updateResp.Updates {
			err := postDetailToThread(slackClient, conf.Channel, mainThreadTs, item)
			if err != nil {
				log.Printf("Failed to post detail to thread: %v", err)
			}
			// Rate limit protection
			time.Sleep(time.Second * 1)
		}

		processedCount++
		log.Printf("Successfully processed product: %s", productName)

		// 製品間の間隔
		time.Sleep(time.Second * 2)
	}

	log.Printf("Successfully processed %d products with %d total release notes!", processedCount, len(notes))
	log.Println("All tasks completed!")
}

func summarizeReleaseNotes(ctx context.Context, client *genai.Client, modelName string, notes []ReleaseNote, targetDate string, productName string) (*UpdateResponse, error) {
	if len(notes) == 0 {
		return &UpdateResponse{
			ServiceName: productName,
			UpdatesDate: targetDate,
			Updates:     []UpdateItem{},
		}, nil
	}

	// リリースノートをJSON形式に整形してGeminiに渡す
	notesJSON, err := json.MarshalIndent(notes, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal release notes: %v", err)
	}

	prompt := fmt.Sprintf(`以下はGoogle Cloudの%s（%s）のリリースノート情報です。
これらを分析して、各リリースノートを要約し、指定されたJSON形式で出力してください。

リリースノートデータ:
%s

## 指示
1. 各リリースノートを個別に処理してください（まとめたり省略しないでください）
2. descriptionを読んで、わかりやすい日本語のタイトルを生成してください
3. summaryは各リリースノートの要点を箇条書きで3点程度にまとめてください
4. serviceフィールドには "%s" を設定してください
5. linkは "https://cloud.google.com/release-notes" を設定してください
6. 全て自然な日本語で出力してください

## JSON Schema
{
  "service_name": "%s",
  "updates_date": "%s",
  "updates": [
    {
      "service": "%s",
      "title": "string (descriptionから生成した日本語タイトル)",
      "date": "%s",
      "summary": ["要点1", "要点2", "要点3"],
      "link": "https://cloud.google.com/release-notes"
    }
  ]
}
`, productName, targetDate, string(notesJSON), productName, productName, targetDate, productName, targetDate)

	cfg := &genai.GenerateContentConfig{
		ResponseMIMEType: "application/json",
	}

	result, err := client.Models.GenerateContent(ctx, modelName, genai.Text(prompt), cfg)
	if err != nil {
		return nil, err
	}

	if len(result.Candidates) == 0 || result.Candidates[0].Content == nil || len(result.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("no content generated")
	}

	var resp UpdateResponse
	err = json.Unmarshal([]byte(result.Candidates[0].Content.Parts[0].Text), &resp)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %v, raw: %s", err, result.Candidates[0].Content.Parts[0].Text)
	}

	return &resp, nil
}

func postDetailToThread(api *slack.Client, channel, threadTs string, item UpdateItem) error {
	// タイトルと日付
	titleText := fmt.Sprintf("*%s*\n`%s`", item.Title, item.Date)

	// サマリーを箇条書きに
	var summaryItems []string
	for _, s := range item.Summary {
		summaryItems = append(summaryItems, fmt.Sprintf("• %s", s))
	}
	summaryText := strings.Join(summaryItems, "\n")

	// Blocksを使って構造化
	detailBlocks := []slack.Block{
		slack.NewSectionBlock(
			slack.NewTextBlockObject("mrkdwn", titleText, false, false),
			nil,
			nil,
		),
		slack.NewSectionBlock(
			slack.NewTextBlockObject("mrkdwn", summaryText, false, false),
			nil,
			nil,
		),
		slack.NewContextBlock(
			"",
			slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("<%s|詳細を見る>", item.Link), false, false),
		),
	}

	_, _, err := api.PostMessage(channel,
		slack.MsgOptionBlocks(detailBlocks...),
		slack.MsgOptionTS(threadTs),
		slack.MsgOptionPostMessageParameters(slack.PostMessageParameters{
			UnfurlLinks: false,
			UnfurlMedia: false,
		}),
	)
	return err
}
