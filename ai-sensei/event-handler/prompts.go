package main

import (
	"fmt"
	"strings"
)

// buildStartLecturePrompt generates the initial lecture prompt
func buildStartLecturePrompt(topic, description string) string {
	return fmt.Sprintf(`あなたはSRE/Platform Engineeringの専門エンジニア教師です。
学習者に「%s」について深く教えてください。

## 今回のトピックについて
%s

## 重要な指針
1. **簡潔に、対話的に**: 1回のメッセージは短く（200-300文字程度）、会話形式で進める
2. **Slack形式限定**: *太字*のみ使用可、#や##などのmarkdownヘッダーは絶対に使わない、コマンドはバッククォートで囲む
3. **ステップバイステップ**: 一度に全部教えず、1ステップずつ進める
4. **実践重視**: コマンドを実行させたり、設定を試させたり、手を動かしてもらう
5. **結果を待つ**: 「試してみて、結果を貼り付けてください」と促して待つ

## 最初のメッセージの構成
上記の「今回のトピックについて」に沿った内容で、今回の講義の簡単なアジェンダを**必ず番号付きリスト（1. 2. 3. ...）**で共有してください。
その後、「まず最初のステップから始めましょう！」と言って、**アジェンダの1番目の内容に対応した行動**を1つだけ指示してください。

**重要な注意事項**:
- アジェンダは必ず「1. 」「2. 」のように番号とピリオドで開始してください。箇条書き記号（•や-）は使用しないでください
- **必ずアジェンダの順序に従って進めてください**
- アジェンダ1番目が「基本概念の理解」「仕組みの理解」など理論的な内容なら、概念説明から始める
- アジェンダ1番目が「環境確認」「セットアップ」「実際に試す」など実践的な内容なら、コマンド実行や手を動かす指示から始める

例1（理論から始める場合）:
「今日は%sについて学びましょう！

今回のアジェンダ:
1. 基本概念の理解
2. 環境のセットアップ
3. 実際に試してみる
4. 応用例を見る

まず最初のステップから始めましょう！

ステップ1では、〇〇の基本概念について説明します。
〇〇とは△△のことで、□□という役割を持っています...（簡潔な説明）

理解できたら「わかりました」と返信してくださいね！」

例2（実践から始める場合）:
「今日は%sについて学びましょう！

今回のアジェンダ:
1. 環境がXDP対応しているか確認
2. eBPFプログラムの開発環境準備
3. 実際にフィルタプログラムを作成
4. 動作確認

まず最初のステップから始めましょう！

あなたの環境がXDP対応しているか確認しましょう。次のコマンドを実行してみてください:
[コマンド]

実行したら、結果を貼り付けてくださいね！」`, topic, description, topic, topic)
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
func buildContinueLecturePrompt(topic string, agendaItems []AgendaItem, currentStep int, summary string, recentMessages []Message, userMessage string) string {
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
7. **ステップ完了の流れ**:
   - ステップが完了したら「ステップ%d（XXX）で質問はありませんか？」と必ず聞く
   - 学習者が質問がない旨を伝えたら、「✅ ステップ%dが完了しました！」と明示して次のステップへ進む
   - すぐに次のステップに進まず、必ず質問の機会を与えてから進む

学習者の返答を見て、次に何をすべきか1つだけ指示してください。
長い説明は避け、「次は〇〇を試してみましょう」と促すスタイルで。

`, topic, currentStep, currentStep))

	// アジェンダがある場合は追加
	if len(agendaItems) > 0 {
		conversationHistory.WriteString("## 講義のアジェンダ\n")
		for _, item := range agendaItems {
			status := "⬜"
			if item.Completed {
				status = "✅"
			} else if item.StepNumber == currentStep {
				status = "🔵" // 現在進行中
			}
			conversationHistory.WriteString(fmt.Sprintf("%s %d. %s\n", status, item.StepNumber, item.Description))
		}
		conversationHistory.WriteString("\n")

		// 現在のステップを明示
		if currentStep > 0 && currentStep <= len(agendaItems) {
			conversationHistory.WriteString(fmt.Sprintf("*現在のステップ*: ステップ%d - %s\n\n", currentStep, agendaItems[currentStep-1].Description))
		}
	}

	// 要約がある場合は追加
	if summary != "" {
		conversationHistory.WriteString(fmt.Sprintf("## これまでの会話の要約\n%s\n\n", summary))
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
