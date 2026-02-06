package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/slack-go/slack"
	"golang.org/x/net/html"
	"google.golang.org/genai"

	"github.com/ureuzy/cloud_functions/ai-web-summarizer/config"
)

func main() {
	ctx := context.Background()
	conf, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	client := resty.New()
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

		// 1. Fetch Web Page
		log.Printf("Fetching content from: %s", targetURL)
		resp, err := client.R().Get(targetURL)
		if err != nil {
			log.Printf("Failed to fetch URL %s: %v", targetURL, err)
			continue
		}

		// 2. Extract Text from HTML
		bodyText := extractText(resp.String())
		if len(bodyText) > 10000 {
			bodyText = bodyText[:10000]
		}

		// 3. AI Translation and Summary
		log.Printf("Summarizing with Gemini...")
		summary, err := summarize(ctx, genaiClient, conf.ModelName, bodyText)
		if err != nil {
			log.Printf("Failed to summarize %s: %v", targetURL, err)
			continue
		}

		// 4. Send to Slack
		log.Printf("Sending to Slack...")
		err = sendToSlack(conf, targetURL, summary)
		if err != nil {
			log.Printf("Failed to send to Slack for %s: %v", targetURL, err)
			continue
		}

		log.Printf("Successfully processed: %s", targetURL)

		// レート制限(429)を回避するためにウェイトを置く
		log.Println("Waiting for next request...")
		time.Sleep(time.Second * 10)
	}

	log.Println("All tasks completed!")
}

func extractText(htmlStr string) string {
	doc, err := html.Parse(strings.NewReader(htmlStr))
	if err != nil {
		return htmlStr
	}

	var f func(*html.Node)
	var sb strings.Builder
	f = func(n *html.Node) {
		if n.Type == html.TextNode {
			data := strings.TrimSpace(n.Data)
			if data != "" {
				sb.WriteString(data + " ")
			}
		}
		if n.Type == html.ElementNode && (n.Data == "script" || n.Data == "style") {
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)
	return sb.String()
}

func summarize(ctx context.Context, client *genai.Client, modelName, text string) (string, error) {
	prompt := fmt.Sprintf(`以下はアップデート情報ページです。今日のアップデート内容を日本語で要約してください。
## 制約事項:
- Slackの "mrkdwn" 形式を使用してください。
- 見出し（#）は使わず、太字（*テキスト*）を使用してください。
- アップデート項目（クラウドサービス）毎に日時を記載してください。
- アップデートは本日の内容だけで良いです。
- 時間は日本時間（JST）で換算してください
- アップデート項目の間はわかりやすいように罫線をいれてください。
- 各アップデート情報のセクションへのリンクも記載してください。
- アップデート情報の日付を必ず確認して、余計な情報は出さないでください。
- アップデート情報を読んでどのサービスのアップデートなのかを最初に出力してください(Google Cloud, AWS, Kubernetesなど)。
- アップデート情報がなければ「アップデート情報なし」と記載してください。
- アップデート情報がある場合は「本日のアップデートはn件です」と表示してください。
- 要点を箇書き（•）で3点程度にまとめてください。
- 翻訳は自然な日本語で行ってください。

内容:
%s`, text)

	result, err := client.Models.GenerateContent(ctx, modelName, genai.Text(prompt), nil)
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
