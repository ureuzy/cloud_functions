package main

import (
	"fmt"
	"strings"
)

// buildStartLecturePrompt generates the initial lecture prompt
func buildStartLecturePrompt(topic string) string {
	return fmt.Sprintf(`あなたはSRE/Platform Engineeringの専門エンジニア教師です。
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
}

// buildSummarizePrompt generates the prompt for summarizing messages
func buildSummarizePrompt(topic string, messages []Message) string {
	// メッセージ履歴を構築
	var messageText strings.Builder
	for _, msg := range messages {
		role := "学習者"
		if msg.Role == "model" {
			role = "教師"
		}
		messageText.WriteString(fmt.Sprintf("%s: %s\n", role, msg.Content))
	}

	return fmt.Sprintf(`以下は「%s」に関する学習セッションの会話履歴です。
この会話を簡潔に要約してください。要約は後続の会話で文脈を理解するために使用されます。

## 会話履歴
%s

## 要約の要件
- 主要なポイントと学習内容を簡潔にまとめる
- 200文字以内で要約する
- 学習者の理解度や進捗状況を含める`, topic, messageText.String())
}

// buildContinueLecturePrompt generates the prompt for continuing the lecture
func buildContinueLecturePrompt(topic, summary string, recentMessages []Message, userMessage string) string {
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

学習者の返答を見て、次に何をすべきか1つだけ指示してください。
長い説明は避け、「次は〇〇を試してみましょう」と促すスタイルで。

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

	return conversationHistory.String()
}
