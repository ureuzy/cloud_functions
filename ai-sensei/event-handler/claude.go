package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

type ClaudeClient struct {
	client    anthropic.Client
	modelName string
}

func NewClaudeClient(ctx context.Context, apiKey, modelName string) (*ClaudeClient, error) {
	client := anthropic.NewClient(
		option.WithAPIKey(apiKey),
	)

	return &ClaudeClient{
		client:    client,
		modelName: modelName,
	}, nil
}

// StartLecture generates the first lecture message for a topic
func (c *ClaudeClient) StartLecture(ctx context.Context, topic string) (string, error) {
	prompt := fmt.Sprintf(`あなたはSRE/Platform Engineeringの専門エンジニア教師です。
学習者に「%s」について深く教えてください。

## 重要な指針
1. **簡潔に、対話的に**: 1回のメッセージは短く（200-300文字程度）、会話形式で進める
2. **Slack形式限定**: *太字*のみ使用可、#や##などのmarkdownヘッダーは絶対に使わない、コマンドはバッククォートで囲む
3. **ステップバイステップ**: 一度に全部教えず、1ステップずつ進める
4. **実践重視**: コマンドを実行させたり、設定を試させたり、手を動かしてもらう
5. **結果を待つ**: 「試してみて、結果を貼り付けてください」と促して待つ

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

実行したら、結果を貼り付けてくださいね！」`, topic, topic)

	message, err := c.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(c.modelName),
		MaxTokens: 1024,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate content: %w", err)
	}

	if len(message.Content) == 0 {
		return "", fmt.Errorf("no content generated")
	}

	// Extract text from content blocks
	var result strings.Builder
	for _, block := range message.Content {
		if block.Type == "text" {
			result.WriteString(block.Text)
		}
	}

	return result.String(), nil
}

// SummarizeMessages summarizes old messages for context compression
func (c *ClaudeClient) SummarizeMessages(ctx context.Context, topic string, messages []Message) (string, error) {
	if len(messages) == 0 {
		return "", nil
	}

	// メッセージ履歴を構築
	var messageText strings.Builder
	for _, msg := range messages {
		role := "学習者"
		if msg.Role == "model" {
			role = "教師"
		}
		messageText.WriteString(fmt.Sprintf("%s: %s\n", role, msg.Content))
	}

	prompt := fmt.Sprintf(`以下は「%s」に関する学習セッションの会話履歴です。
この会話を簡潔に要約してください。要約は後続の会話で文脈を理解するために使用されます。

## 会話履歴
%s

## 要約の要件
- 主要なポイントと学習内容を簡潔にまとめる
- 200文字以内で要約する
- 学習者の理解度や進捗状況を含める`, topic, messageText.String())

	message, err := c.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(c.modelName),
		MaxTokens: 512,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate summary: %w", err)
	}

	if len(message.Content) == 0 {
		return "", fmt.Errorf("no summary generated")
	}

	// Extract text from content blocks
	var result strings.Builder
	for _, block := range message.Content {
		if block.Type == "text" {
			result.WriteString(block.Text)
		}
	}

	return result.String(), nil
}

// ContinueLecture continues the conversation based on history
func (c *ClaudeClient) ContinueLecture(ctx context.Context, topic, summary string, recentMessages []Message, userMessage string) (string, error) {
	// システムプロンプト
	systemPrompt := fmt.Sprintf(`あなたはSRE/Platform Engineeringの専門エンジニア教師です。
「%s」について学習者に深く教えています。

## 重要な指針
1. **簡潔に、対話的に**: 1回のメッセージは短く（200-300文字程度）、会話形式で進める
2. **Slack形式限定**: *太字*のみ使用可、#や##などのmarkdownヘッダーは絶対に使わない、コマンドはバッククォートで囲む
3. **ステップバイステップ**: 一度に全部教えず、1ステップずつ進める
4. **実践重視**: コマンドを実行させたり、設定を試させたり、手を動かしてもらう
5. **結果を確認**: 学習者が貼り付けた結果を確認し、次のステップに進む
6. **質問に答える**: 学習者の質問には丁寧に答え、理解度を確認してから次へ

学習者の返答を見て、次に何をすべきか1つだけ指示してください。
長い説明は避け、「次は〇〇を試してみましょう」と促すスタイルで。`, topic)

	// Build conversation messages
	var conversationMessages []anthropic.MessageParam

	// Add summary as first user message if exists
	if summary != "" {
		conversationMessages = append(conversationMessages, anthropic.NewUserMessage(
			anthropic.NewTextBlock(fmt.Sprintf("これまでの会話の要約:\n%s", summary)),
		))
		conversationMessages = append(conversationMessages, anthropic.NewAssistantMessage(
			anthropic.NewTextBlock("了解しました。要約を確認しました。"),
		))
	}

	// Add recent messages
	for _, msg := range recentMessages {
		if msg.Role == "user" {
			conversationMessages = append(conversationMessages, anthropic.NewUserMessage(
				anthropic.NewTextBlock(msg.Content),
			))
		} else {
			conversationMessages = append(conversationMessages, anthropic.NewAssistantMessage(
				anthropic.NewTextBlock(msg.Content),
			))
		}
	}

	// Add current user message
	conversationMessages = append(conversationMessages, anthropic.NewUserMessage(
		anthropic.NewTextBlock(userMessage),
	))

	message, err := c.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(c.modelName),
		MaxTokens: 1024,
		System: []anthropic.TextBlockParam{
			{Text: systemPrompt, Type: "text"},
		},
		Messages: conversationMessages,
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate content: %w", err)
	}

	if len(message.Content) == 0 {
		return "", fmt.Errorf("no content generated")
	}

	// Extract text from content blocks
	var result strings.Builder
	for _, block := range message.Content {
		if block.Type == "text" {
			result.WriteString(block.Text)
		}
	}

	return result.String(), nil
}
