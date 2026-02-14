# AI Sensei - Event Handler

## 概要
SlackのInteractive ComponentsとEvents APIを処理し、Gemini/Claudeで対話型技術講義を提供するCloud Run Serviceです。会話履歴をFirestoreで管理し、継続的な学習体験を実現します。

## 主な機能
- Slackボタンクリックの処理（学習開始・スキップ）
- Slackスレッド内メッセージの処理
- Gemini/Claudeによる対話型講義の生成
- Firestoreでの会話履歴管理（要約 + 最新10件詳細）
- Slack署名検証によるセキュリティ確保

## 環境変数

| 環境変数名 | 必須 | デフォルト値 | 説明 |
|-----------|------|-------------|------|
| `SLACK_BOT_TOKEN` | ✓ | - | Slack Bot User OAuth Token (xoxb-で始まるトークン) |
| `SLACK_SIGNING_SECRET` | ✓ | - | Slackアプリの署名検証用シークレット |
| `PROJECT_ID` | ✓ | - | Vertex AI & Firestore用のGoogle Cloud Project ID |
| `LOCATION` | | `global` | Vertex AIのロケーション |
| `AI_PROVIDER` | | `gemini` | AIプロバイダー (`gemini` または `claude`) |
| `MODEL_NAME` | | `gemini-3-flash-preview` | 使用するモデル名 |
| `CLAUDE_API_KEY` | | - | Claude APIキー（AI_PROVIDER=claudeの場合必須） |
| `PORT` | | `8080` | サーバーのポート番号 |
| `MAX_RECENT_MESSAGES` | | `10` | 詳細保持する最新メッセージ数 |

## 依存関係

### Google Cloud サービス
- **Vertex AI API**: Geminiによる講義生成
- **Firestore**: 会話履歴の保存・取得

### 外部サービス
- **Slack**: イベント受信、メッセージ送信
- **Anthropic Claude API**: Claudeによる講義生成（オプション）

### 必要な権限
- `roles/aiplatform.user` - Vertex AI使用権限
- `roles/datastore.user` - Firestore読み書き権限
- Slack Bot Scopes: `chat:write`, `app_mentions:read`, `channels:history`, `groups:history`
- Slack Events: `app_mention`, `message.channels`

## デプロイ方法

### 前提条件
```bash
# Vertex AIを有効化
gcloud services enable aiplatform.googleapis.com

# Firestoreを有効化
gcloud services enable firestore.googleapis.com
```

### Slack Appの設定

#### Event Subscriptions
- Request URL: `https://your-event-handler-url/slack/events`
- Subscribe to bot events:
  - `app_mention`
  - `message.channels`

#### Interactive Components
- Request URL: `https://your-event-handler-url/slack/interactive`

### デプロイコマンド
```bash
# イメージをビルド
gcloud builds submit --tag gcr.io/your-project/ai-sensei-event-handler

# Cloud Run Serviceとしてデプロイ
gcloud run deploy ai-sensei-event-handler \
  --image gcr.io/your-project/ai-sensei-event-handler \
  --region asia-northeast1 \
  --set-env-vars PROJECT_ID=your-project \
  --set-env-vars LOCATION=global \
  --set-env-vars AI_PROVIDER=gemini \
  --set-env-vars MAX_RECENT_MESSAGES=10 \
  --set-secrets SLACK_BOT_TOKEN=slack-sensei-bot:latest \
  --set-secrets SLACK_SIGNING_SECRET=slack-signing-secret:latest \
  --allow-unauthenticated
```

## エンドポイント

| パス | メソッド | 説明 |
|------|---------|------|
| `/healthz` | GET | ヘルスチェック |
| `/slack/events` | POST | Slack Events API |
| `/slack/interactive` | POST | Slack Interactive Components |

## 会話履歴管理

### Firestoreデータ構造
```
threads/{thread_ts}
  ├── topic: "トピック名"
  ├── summary: "過去の会話要約"
  ├── recent_messages: [最新10件の詳細メッセージ]
  ├── created_at: Timestamp
  └── updated_at: Timestamp
```

### メモリ管理戦略
- 最新10件のメッセージは詳細に保持
- それ以前の会話は要約して圧縮（将来実装予定）
- AIリクエスト: システムプロンプト + 要約 + 最新10件 + 新メッセージ

## 動作フロー

### 学習開始時
1. ユーザーが「学習を開始」ボタンをクリック
2. Event HandlerがInteractive Componentsを受信
3. 新しいスレッドを作成してFirestoreに保存
4. Gemini/Claudeで初回講義を生成
5. Slackスレッドに投稿

### 会話継続時
1. ユーザーがスレッドにメッセージを投稿
2. Event HandlerがSlack Eventsを受信
3. Firestoreから会話履歴を取得
4. 履歴とともにGemini/Claudeにリクエスト
5. 生成された応答をスレッドに投稿
6. 会話履歴をFirestoreに更新

## Claude API使用時の設定
```bash
# Claudeを使用する場合
gcloud run deploy ai-sensei-event-handler \
  --update-env-vars AI_PROVIDER=claude \
  --update-env-vars MODEL_NAME=claude-opus-4-6 \
  --set-secrets CLAUDE_API_KEY=claude-api-key:latest
```
