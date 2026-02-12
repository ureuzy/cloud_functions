package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"math/rand"
	"strings"
	"time"

	"google.golang.org/api/googleapi"
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

// retryWithExponentialBackoff retries a function with exponential backoff on 429 errors
func retryWithExponentialBackoff(ctx context.Context, maxRetries int, fn func() error) error {
	for attempt := 0; attempt < maxRetries; attempt++ {
		err := fn()
		if err == nil {
			return nil
		}

		// Check if it's a 429 error
		if apiErr, ok := err.(*googleapi.Error); ok && apiErr.Code == 429 {
			if attempt < maxRetries-1 {
				backoff := time.Duration(math.Pow(2, float64(attempt))) * time.Second
				jitter := time.Duration(rand.Int63n(int64(time.Second)))
				waitDuration := backoff + jitter
				log.Printf("Received 429 error, retrying after %v (attempt %d/%d)", waitDuration, attempt+1, maxRetries)
				time.Sleep(waitDuration)
				continue
			}
		}

		return err
	}

	return fmt.Errorf("max retries exceeded")
}

// StartLecture generates the first lecture message for a topic
func (g *GeminiClient) StartLecture(ctx context.Context, topic string) (string, error) {
	prompt := fmt.Sprintf(`あなたはSRE/Platform Engineeringの専門エンジニア教師です。
学習者に「%s」について深く教えてください。

## 重要な指針
1. **簡潔に、対話的に**: 1回のメッセージは短く（200-300文字程度）、会話形式で進める
2. **Slack形式限定**: *太字*のみ使用可、#や##などのmarkdownヘッダーは絶対に使わない、コマンドはバッククォートで囲む
3. **ステップバイステップ**: 一度に全部教えず、1ステップずつ進める
4. **実践重視**: コマンドを実行させたり、設定を試させたり、手を動かしてもらう
5. **結果を待つ**: 「試してみて、結果を貼り付けてください」と促して待つ
6. **質問タイムを設ける**: 各ステップの後に必ず「ここまでで質問はありますか？」と尋ねる

## 最初のメッセージの構成
まず今回の講義の簡単なアジェンダ（3-5ステップ）を箇条書きで共有してください。
その後、「まず最初のステップから始めましょう！」と言って、具体的に何をすべきか1つだけ指示してください。

例:
「今日は%sについて学びましょう！

今回のアジェンダ:
1. 基本概念の理解
2. 実際に試してみる
3. 動作確認
4. 応用例を見る

まず最初のステップから始めましょう！
〇〇を確認するために、次のコマンドを実行してみてください:
[コマンド]

実行したら、結果を貼り付けてくださいね！

ここまでで質問はありますか？」`, topic, topic)

	var result *genai.GenerateContentResponse
	err := retryWithExponentialBackoff(ctx, 5, func() error {
		var genErr error
		result, genErr = g.client.Models.GenerateContent(ctx, g.modelName, genai.Text(prompt), nil)
		return genErr
	})
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

## 重要な指針
1. **簡潔に、対話的に**: 1回のメッセージは短く（200-300文字程度）、会話形式で進める
2. **Slack形式限定**: *太字*のみ使用可、#や##などのmarkdownヘッダーは絶対に使わない、コマンドはバッククォートで囲む
3. **ステップバイステップ**: 一度に全部教えず、1ステップずつ進める
4. **実践重視**: コマンドを実行させたり、設定を試させたり、手を動かしてもらう
5. **結果を確認**: 学習者が貼り付けた結果を確認し、次のステップに進む
6. **質問に答える**: 学習者の質問には丁寧に答え、理解度を確認してから次へ
7. **質問タイムを設ける**: 各ステップの後に必ず「ここまでで質問はありますか？」と尋ねる

学習者の返答を見て、次に何をすべきか1つだけ指示してください。
長い説明は避け、「次は〇〇を試してみましょう」と促すスタイルで。
各ステップの最後には必ず「ここまでで質問はありますか？」と追加してください。

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

	var result *genai.GenerateContentResponse
	err := retryWithExponentialBackoff(ctx, 5, func() error {
		var genErr error
		result, genErr = g.client.Models.GenerateContent(ctx, g.modelName, genai.Text(conversationHistory.String()), nil)
		return genErr
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate content: %v", err)
	}

	if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no content generated")
	}

	return result.Candidates[0].Content.Parts[0].Text, nil
}
