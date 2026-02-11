# AI Sensei - 技術学習支援Bot

SRE/Platform Engineeringの深い技術トピックを毎日提供し、Geminiによる対話型講義を行うSlack Botです。

## アーキテクチャ

### 1. Daily Poster (Cloud Run Job)
- 毎日9:00 JSTに実行
- Geminiに技術トピックを選定してもらう
- Slackにボタン付きメッセージを投稿

### 2. Event Handler (Cloud Run Service)
- Slack Interactive Componentsを処理（ボタンクリック）
- Slack Events APIを処理（スレッド内メッセージ）
- Firestoreで会話履歴を管理（要約 + 最新10件詳細）
- Geminiで講義を生成・継続

## 会話履歴管理

### 要約ベースアプローチ
```
Firestore:
├── Thread Document
│   ├── summary: "過去の会話要約"
│   ├── recent_messages: [最新10件の詳細]
│   ├── topic: "トピック名"
│   └── timestamps
```

- 最新10件のメッセージは詳細に保持
- それ以前の会話は要約して圧縮（将来実装）
- Geminiへのリクエスト: システムプロンプト + 要約 + 最新10件 + 新メッセージ

## セットアップ

### 前提条件
- Google Cloud Project (Vertex AI, Firestore, Cloud Run有効化)
- Slack App (Bot Token, Signing Secret, Interactive Components, Events API)

### 1. Slack App設定

#### Bot Token Scopes
- `chat:write`
- `app_mentions:read`
- `channels:history`
- `groups:history`

#### Event Subscriptions
- Request URL: `https://your-event-handler-url/slack/events`
- Subscribe to bot events:
  - `app_mention`
  - `message.channels` (Botが参加しているチャンネル)

#### Interactive Components
- Request URL: `https://your-event-handler-url/slack/interactive`

### 2. Google Cloud Secretsに保存
```bash
# Slack Bot Token
echo -n "xoxb-your-token" | gcloud secrets create slack-ai-bot --data-file=-

# Slack Signing Secret
echo -n "your-signing-secret" | gcloud secrets create slack-signing-secret --data-file=-
```

### 3. Service Accountの作成と権限付与
```bash
# Service Account作成
gcloud iam service-accounts create ai-sensei \
  --display-name="AI Sensei Bot"

# Vertex AI User権限
gcloud projects add-iam-policy-binding ureuzy-ai \
  --member="serviceAccount:ai-sensei@ureuzy-common.iam.gserviceaccount.com" \
  --role="roles/aiplatform.user"

# Firestore User権限
gcloud projects add-iam-policy-binding ureuzy-ai \
  --member="serviceAccount:ai-sensei@ureuzy-common.iam.gserviceaccount.com" \
  --role="roles/datastore.user"

# Secret Accessor権限
gcloud secrets add-iam-policy-binding slack-ai-bot \
  --member="serviceAccount:ai-sensei@ureuzy-common.iam.gserviceaccount.com" \
  --role="roles/secretmanager.secretAccessor"

gcloud secrets add-iam-policy-binding slack-signing-secret \
  --member="serviceAccount:ai-sensei@ureuzy-common.iam.gserviceaccount.com" \
  --role="roles/secretmanager.secretAccessor"
```

### 4. デプロイ

#### Daily Poster
```bash
cd daily-poster

# ビルド
gcloud builds submit --tag gcr.io/ureuzy-ai/ai-sensei-daily-poster

# デプロイ
gcloud run jobs create ai-sensei-daily-poster \
  --image gcr.io/ureuzy-ai/ai-sensei-daily-poster \
  --region asia-northeast1 \
  --service-account ai-sensei@ureuzy-common.iam.gserviceaccount.com \
  --set-env-vars CHANNEL=#ai-sensei,PROJECT_ID=ureuzy-ai,LOCATION=global,GEMINI_MODEL=gemini-1.5-pro \
  --set-secrets SLACK_BOT_TOKEN=slack-ai-bot:latest

# スケジュール設定（毎日9:00 JST）
gcloud scheduler jobs create http ai-sensei-daily \
  --location asia-northeast1 \
  --schedule "0 9 * * *" \
  --time-zone "Asia/Tokyo" \
  --uri "https://asia-northeast1-run.googleapis.com/apis/run.googleapis.com/v1/namespaces/ureuzy-ai/jobs/ai-sensei-daily-poster:run" \
  --http-method POST \
  --oauth-service-account-email ai-sensei@ureuzy-common.iam.gserviceaccount.com
```

#### Event Handler
```bash
cd event-handler

# ビルド
gcloud builds submit --tag gcr.io/ureuzy-ai/ai-sensei-event-handler

# デプロイ
gcloud run deploy ai-sensei-event-handler \
  --image gcr.io/ureuzy-ai/ai-sensei-event-handler \
  --region asia-northeast1 \
  --service-account ai-sensei@ureuzy-common.iam.gserviceaccount.com \
  --set-env-vars PROJECT_ID=ureuzy-ai,LOCATION=global,GEMINI_MODEL=gemini-1.5-pro,MAX_RECENT_MESSAGES=10 \
  --set-secrets SLACK_BOT_TOKEN=slack-ai-bot:latest,SLACK_SIGNING_SECRET=slack-signing-secret:latest \
  --allow-unauthenticated
```

### 5. Slack AppのRequest URLを更新
Event HandlerのURLを取得して、Slack AppのEvent SubscriptionsとInteractive ComponentsのRequest URLを更新する。

## コスト見積もり（月間100スレッド想定）

- **Gemini API**: $1.28/月
  - Daily Poster: $0.03/月（トピック選定）
  - 講義スレッド: $1.25/月
- **Firestore**: $0.006/月
- **Cloud Run**: $0.05/月

**合計: 約 $1.34/月**

## 使い方

1. 毎日9:00に `#ai-sensei` チャンネルに技術トピックが投稿される
2. 「学習を開始」ボタンをクリックするとスレッドで講義が始まる
3. スレッド内で質問・会話を続けることで深く学べる
4. 会話履歴は自動的にFirestoreに保存される

## トラブルシューティング

### Event Handlerが反応しない
- Slack AppのEvent SubscriptionsのRequest URLが正しいか確認
- Cloud Runのログでエラーをチェック: `gcloud run logs read ai-sensei-event-handler --region asia-northeast1`

### トピックが投稿されない
- Cloud Scheduler Jobが有効か確認: `gcloud scheduler jobs describe ai-sensei-daily --location asia-northeast1`
- Cloud Run Jobのログをチェック: `gcloud run jobs logs ai-sensei-daily-poster --region asia-northeast1`

### Firestoreエラー
- Service AccountにFirestore User権限があるか確認
- Firestoreがプロジェクトで有効化されているか確認
