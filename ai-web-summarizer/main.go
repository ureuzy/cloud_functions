package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/slack-go/slack"
	"google.golang.org/genai"

	"github.com/ureuzy/cloud_functions/ai-web-summarizer/config"
)

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

	for _, targetURL := range conf.TargetURLs {
		targetURL = strings.TrimSpace(targetURL)
		if targetURL == "" {
			continue
		}

		log.Printf("--- Processing: %s ---", targetURL)

		log.Printf("Fetching and summarizing with Gemini...")
		updateResp, err := fetchUpdates(ctx, genaiClient, conf.ModelName, targetURL)
		if err != nil {
			log.Printf("Failed to fetch updates for %s: %v", targetURL, err)
			continue
		}

		if len(updateResp.Updates) == 0 {
			msg := fmt.Sprintf("[%s] %s のアップデート情報はありません。", updateResp.ServiceName, updateResp.UpdatesDate)
			_, _, err := slackClient.PostMessage(conf.Channel, slack.MsgOptionText(msg, false))
			if err != nil {
				log.Printf("Failed to send 'no updates' message: %v", err)
			}
			continue
		}

		// 1. Post Header and List
		parentTs, err := postHeader(slackClient, conf.Channel, targetURL, updateResp)
		if err != nil {
			log.Printf("Failed to post header: %v", err)
			continue
		}

		// 2. Post Details to Thread
		for _, item := range updateResp.Updates {
			err := postDetailToThread(slackClient, conf.Channel, parentTs, item)
			if err != nil {
				log.Printf("Failed to post detail to thread: %v", err)
			}
			// Rate limit protection
			time.Sleep(time.Second * 1)
		}

		log.Printf("Successfully processed: %s", targetURL)
		time.Sleep(time.Second * 5)
	}

	log.Println("All tasks completed!")
}

func fetchUpdates(ctx context.Context, client *genai.Client, modelName, targetURL string) (*UpdateResponse, error) {
	jst := time.FixedZone("Asia/Tokyo", 9*60*60)
	yesterday := time.Now().In(jst).AddDate(0, 0, -1)
	targetDateStr := yesterday.Format("2006/01/02")

	prompt := fmt.Sprintf(`以下のURLにアクセスし、内容を読み取って【対象日: %s (JST)】のアップデート情報を抽出してください。
URL: %s

## 指示（最優先）
1. 指定されたURLの内容（特に日付）を隅々まで確認し、対象日（%s）と一致するアップデートを【全て、1件も漏らさずに】抽出してください。
2. 項目が多数ある場合でも、勝手に要約したり、上位数件に絞ったりせず、該当するものは全て抽出してください。
3. 機能やサービス毎に個別に抽出してください。勝手にまとめたり省略しないでください。
4. 対象日（%s）以外の情報は完全に無視してください。
5. サービス名はサイト全体が何についてのものか（Google Cloud, AWSなど）を回答してください。

## 出力形式
- 結果は必ず指定されたJSON形式で出力してください。
- summaryは各アップデートの要点を箇条書きで3点程度にまとめてください。
- 翻訳は自然な日本語で行ってください。

## JSON Schema
{
  "service_name": "string",
  "updates_date": "string (YYYY/MM/DD)",
  "updates": [
    {
      "service": "string (具体的なサービス名、例: Compute Engine)",
      "title": "string",
      "date": "string (YYYY/MM/DD JST)",
      "summary": ["point 1", "point 2", "point 3"],
      "link": "string"
    }
  ]
}
`, targetDateStr, targetURL, targetDateStr, targetDateStr)

	cfg := &genai.GenerateContentConfig{
		Tools: []*genai.Tool{
			{GoogleSearch: &genai.GoogleSearch{}},
		},
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

func postHeader(api *slack.Client, channel, targetURL string, resp *UpdateResponse) (string, error) {
	headerText := fmt.Sprintf("*AI Update Report: %s*\n%s のアップデートは %d 件です。", resp.ServiceName, resp.UpdatesDate, len(resp.Updates))

	var listItems []string
	for i, item := range resp.Updates {
		listItems = append(listItems, fmt.Sprintf("%d. *%s*: %s", i+1, item.Service, item.Title))
	}

	blocks := []slack.Block{
		slack.NewSectionBlock(slack.NewTextBlockObject("mrkdwn", headerText, false, false), nil, nil),
		slack.NewDividerBlock(),
		slack.NewSectionBlock(slack.NewTextBlockObject("mrkdwn", strings.Join(listItems, "\n"), false, false), nil, nil),
		slack.NewContextBlock("", slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("<%s|Original Source>", targetURL), false, false)),
	}

	_, ts, err := api.PostMessage(channel,
		slack.MsgOptionBlocks(blocks...),
		slack.MsgOptionPostMessageParameters(slack.PostMessageParameters{
			UnfurlLinks: false,
			UnfurlMedia: false,
		}),
	)
	return ts, err
}

func postDetailToThread(api *slack.Client, channel, threadTs string, item UpdateItem) error {
	detailText := fmt.Sprintf("*[%s] %s*\n(%s JST)\n", item.Service, item.Title, item.Date)
	for _, s := range item.Summary {
		detailText += fmt.Sprintf("• %s\n", s)
	}
	detailText += fmt.Sprintf("リンク: %s", item.Link)

	_, _, err := api.PostMessage(channel,
		slack.MsgOptionText(detailText, false),
		slack.MsgOptionTS(threadTs),
		slack.MsgOptionPostMessageParameters(slack.PostMessageParameters{
			UnfurlLinks: false,
			UnfurlMedia: false,
		}),
	)
	return err
}
