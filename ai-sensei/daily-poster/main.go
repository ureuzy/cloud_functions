package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/slack-go/slack"
	"google.golang.org/genai"

	"github.com/ureuzy/cloud_functions/ai-sensei/daily-poster/config"
)

type TopicSuggestion struct {
	Topic       string `json:"topic"`
	Description string `json:"description"`
}

func main() {
	ctx := context.Background()
	conf, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	log.Printf("Starting AI Sensei Daily Poster")

	// Gemini クライアント初期化
	genaiClient, err := genai.NewClient(ctx, &genai.ClientConfig{
		Project:  conf.ProjectID,
		Location: conf.Location,
		Backend:  genai.BackendVertexAI,
	})
	if err != nil {
		log.Fatalf("Failed to create GenAI client: %v", err)
	}

	// Gemini で今日のトピックを選定
	topic, err := selectDailyTopic(ctx, genaiClient, conf.ModelName)
	if err != nil {
		log.Fatalf("Failed to select daily topic: %v", err)
	}

	log.Printf("Selected topic: %s", topic.Topic)

	// Slack に投稿
	slackClient := slack.New(conf.SlackBotToken)
	err = postToSlack(slackClient, conf.Channel, topic)
	if err != nil {
		log.Fatalf("Failed to post to Slack: %v", err)
	}

	log.Println("Daily topic posted successfully!")
}

func selectDailyTopic(ctx context.Context, client *genai.Client, modelName string) (*TopicSuggestion, error) {
	prompt := `あなたはSRE/Platform Engineeringの専門家です。
毎日1つ、学習者が深く学ぶべき技術トピックを提案してください。

## 選定基準
- **非常に具体的で深い技術トピック**であること（広範なトピックは避ける）
- SRE/Platform Engineering/インフラエンジニアに実務で役立つこと
- 実践的な演習が可能なこと

## トピック例（これらに限らず、同レベルの深さのトピックを選定）
- 認証・認可: OIDC, SAML, mTLS, OAuth 2.0 フロー
- コンテナ技術: Linux Namespace, cgroups, overlay filesystem, OCI Image仕様
- ネットワーク: DNS の詳細動作, HTTP/3, QUIC, BGP, VXLAN
- セキュリティ: TLS 1.3 ハンドシェイク, 暗号化アルゴリズム, PKI
- プロトコル: gRPC ストリーミング, WebSocket, Server-Sent Events
- 可観測性: OpenTelemetry, eBPF, メトリクス収集の仕組み
- CI/CD: Tekton Pipeline, ArgoCD Sync, GitOps ワークフロー
- Kubernetes: CNI プラグイン, Admission Webhook, Custom Scheduler
- ストレージ: CSI ドライバ, etcd の RAFT, Distributed Consensus

## 出力形式
{
  "topic": "今日のトピック名（日本語）",
  "description": "このトピックで何を学ぶか（2-3行の説明）"
}

今日のトピックを1つ提案してください。`

	cfg := &genai.GenerateContentConfig{
		ResponseMIMEType: "application/json",
	}

	result, err := client.Models.GenerateContent(ctx, modelName, genai.Text(prompt), cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to generate content: %v", err)
	}

	if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("no content generated")
	}

	var suggestion TopicSuggestion
	err = json.Unmarshal([]byte(result.Candidates[0].Content.Parts[0].Text), &suggestion)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %v, raw: %s", err, result.Candidates[0].Content.Parts[0].Text)
	}

	return &suggestion, nil
}

func postToSlack(slackClient *slack.Client, channel string, topic *TopicSuggestion) error {
	// Slack Block Kit でメッセージを構築
	headerText := fmt.Sprintf("📚 *今日の学習トピック*\n\n*%s*", topic.Topic)
	descriptionText := topic.Description

	blocks := []slack.Block{
		slack.NewSectionBlock(
			slack.NewTextBlockObject("mrkdwn", headerText, false, false),
			nil,
			nil,
		),
		slack.NewSectionBlock(
			slack.NewTextBlockObject("mrkdwn", descriptionText, false, false),
			nil,
			nil,
		),
		slack.NewDividerBlock(),
	}

	// ボタンを追加
	startButton := slack.NewButtonBlockElement(
		"start_learning",
		topic.Topic,
		slack.NewTextBlockObject("plain_text", "学習を開始", true, false),
	)
	startButton.Style = slack.StylePrimary

	skipButton := slack.NewButtonBlockElement(
		"skip_topic",
		topic.Topic,
		slack.NewTextBlockObject("plain_text", "スキップ", true, false),
	)

	actionBlock := slack.NewActionBlock(
		"topic_actions",
		startButton,
		skipButton,
	)

	blocks = append(blocks, actionBlock)

	// メッセージを投稿
	_, _, err := slackClient.PostMessage(
		channel,
		slack.MsgOptionBlocks(blocks...),
		slack.MsgOptionText(fmt.Sprintf("今日のトピック: %s", topic.Topic), false),
	)

	if err != nil {
		return fmt.Errorf("failed to post message: %v", err)
	}

	return nil
}
