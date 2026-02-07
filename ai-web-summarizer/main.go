package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/slack-go/slack"
	"google.golang.org/genai"

	"github.com/ureuzy/cloud_functions/ai-web-summarizer/config"
)

func main() {
	ctx := context.Background()
	conf, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

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

		// Google Searchツールを利用するため、直接URLを渡す
		log.Printf("Summarizing with Gemini (using Google Search tool)...")
		summary, err := summarize(ctx, genaiClient, conf.ModelName, targetURL)
		if err != nil {
			log.Printf("Failed to summarize %s: %v", targetURL, err)
			continue
		}

		// Send to Slack
		log.Printf("Sending to Slack...")
		err = sendToSlack(conf, targetURL, summary)
		if err != nil {
			log.Printf("Failed to send to Slack for %s: %v", targetURL, err)
			continue
		}

		log.Printf("Successfully processed: %s", targetURL)
		time.Sleep(time.Second * 5)
	}

	log.Println("All tasks completed!")
}

func summarize(ctx context.Context, client *genai.Client, modelName, targetURL string) (string, error) {
	// 日本時間 (JST) で前日の日付を基準とする
	jst := time.FixedZone("Asia/Tokyo", 9*60*60)
	yesterday := time.Now().In(jst).AddDate(0, 0, -1)
	targetDateStr := yesterday.Format("2006/01/02")

	prompt := fmt.Sprintf(`
以下のURLにアクセスし、内容を読み取って【対象日: %s (JST)】のアップデート情報を抽出してまとめてください。

## URL 
%s

## 指示
1. 指定されたURLの内容（特に日付）を厳密に確認し、対象日（%s）と一致するアップデートのみを抽出してください。
2. 対象日以外の情報は、たとえ記事内にあっても完全に無視してください。
3. アップデートがある場合は以下のテンプレートで要約してください。
4. 該当するアップデートが1件もない場合は、「%s のアップデート情報はありません。」とだけ出力してください。ただしサービス名は出力してください。
5. Slackの "mrkdwn" 形式を使用し、見出し（#）は使わず太字（*テキスト*）を使用してください。

## 出力テンプレート:
[サービス名 (Google Cloud, AWS, Kubernetesなど)]
%s のアップデートはn件です。
---
*アップデートタイトル* (公開日時/JST)
• 要点1
• 要点2
• 要点3
リンク: [セクションへのリンクURL]
---
`, targetDateStr, targetURL, targetDateStr, targetDateStr, targetDateStr)

	// Google Searchツール (Grounding) を有効にする設定
	cfg := &genai.GenerateContentConfig{
		Tools: []*genai.Tool{
			{GoogleSearch: &genai.GoogleSearch{}},
		},
	}
	result, err := client.Models.GenerateContent(ctx, modelName, genai.Text(prompt), cfg)
	if err != nil {
		return "", err
	}

	if len(result.Candidates) == 0 || result.Candidates[0].Content == nil || len(result.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no content generated")
	}

	var sb strings.Builder
	for _, part := range result.Candidates[0].Content.Parts {
		sb.WriteString(part.Text)
	}

	return sb.String(), nil
}

func sendToSlack(conf *config.Config, targetURL, text string) error {
	headerText := slack.NewTextBlockObject("mrkdwn", "*AI Web Summarizer Report*", false, false)
	headerSection := slack.NewSectionBlock(headerText, nil, nil)

	bodyText := slack.NewTextBlockObject("mrkdwn", text, false, false)
	bodySection := slack.NewSectionBlock(bodyText, nil, nil)

	urlText := slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("<%s|View Original Source>", targetURL), false, false)
	urlSection := slack.NewContextBlock("", urlText)

	blocks := []slack.Block{
		headerSection,
		slack.NewDividerBlock(),
		bodySection,
		slack.NewDividerBlock(),
		urlSection,
	}

	attachment := slack.Attachment{
		Color:  "#4285F4",
		Blocks: slack.Blocks{BlockSet: blocks},
	}

	return slack.PostWebhook(conf.SlackWebhookUrl, &slack.WebhookMessage{
		Username:    "AI Summarizer",
		IconEmoji:   ":robot_face:",
		Channel:     conf.Channel,
		Attachments: []slack.Attachment{attachment},
	})
}
