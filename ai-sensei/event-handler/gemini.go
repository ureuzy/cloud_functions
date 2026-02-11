package main

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/genai"
)

type GeminiClient struct {
	client    *genai.Client
	modelName string
}

func NewGeminiClient(ctx context.Context, projectID, location, modelName string) (*GeminiClient, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		Project:  projectID,
		Location: location,
		Backend:  genai.BackendVertexAI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create GenAI client: %w", err)
	}

	return &GeminiClient{
		client:    client,
		modelName: modelName,
	}, nil
}

// StartLecture generates the first lecture message for a topic
func (g *GeminiClient) StartLecture(ctx context.Context, topic string) (string, error) {
	prompt := fmt.Sprintf(`あなたはSRE/Platform Engineeringの専門エンジニア教師です。
学習者に「%s」について深く教えてください。

## 教え方の指針
1. **深く、実践的に**: 表面的な説明ではなく、内部の仕組みや実装の詳細まで踏み込む
2. **段階的に**: まず基礎概念を説明し、その後に詳細に進む
3. **実例を交える**: 実際のユースケースやコマンド例を示す
4. **質問を促す**: 「ここまでで疑問点はありますか？」など対話を促す
5. **実習を提案**: 学習者が試せる具体的な演習を提案する

まず最初の講義メッセージを生成してください。
学習者の理解度に応じて、次のステップに進んでいきます。`, topic)

	result, err := g.client.Models.GenerateContent(ctx, g.modelName, genai.Text(prompt), nil)
	if err != nil {
		return "", fmt.Errorf("failed to generate content: %v", err)
	}

	if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no content generated")
	}

	return result.Candidates[0].Content.Parts[0].Text, nil
}

// ContinueLecture continues the conversation based on history
func (g *GeminiClient) ContinueLecture(ctx context.Context, topic, summary string, recentMessages []Message, userMessage string) (string, error) {
	// 会話履歴を構築
	var conversationHistory strings.Builder

	// システムプロンプト
	conversationHistory.WriteString(fmt.Sprintf(`あなたはSRE/Platform Engineeringの専門エンジニア教師です。
「%s」について学習者に深く教えています。

## 教え方の指針
1. **深く、実践的に**: 表面的な説明ではなく、内部の仕組みや実装の詳細まで踏み込む
2. **対話的に**: 学習者の質問に丁寧に答え、理解度を確認する
3. **実例を交える**: 実際のユースケースやコマンド例を示す
4. **実習を提案**: 学習者が試せる具体的な演習を提案する

`, topic))

	// 要約がある場合は追加
	if summary != "" {
		conversationHistory.WriteString(fmt.Sprintf("\n## これまでの会話の要約\n%s\n\n", summary))
	}

	// 最近の詳細な会話履歴
	if len(recentMessages) > 0 {
		conversationHistory.WriteString("## 最近の会話\n")
		for _, msg := range recentMessages {
			role := "学習者"
			if msg.Role == "model" {
				role = "あなた（教師）"
			}
			conversationHistory.WriteString(fmt.Sprintf("%s: %s\n\n", role, msg.Content))
		}
	}

	// 新しいユーザーメッセージ
	conversationHistory.WriteString(fmt.Sprintf("学習者: %s\n\n", userMessage))
	conversationHistory.WriteString("あなた（教師）の返答:")

	result, err := g.client.Models.GenerateContent(ctx, g.modelName, genai.Text(conversationHistory.String()), nil)
	if err != nil {
		return "", fmt.Errorf("failed to generate content: %v", err)
	}

	if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no content generated")
	}

	return result.Candidates[0].Content.Parts[0].Text, nil
}
