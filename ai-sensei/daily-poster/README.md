# AI Sensei - Daily Poster

## 概要
毎日SRE/Platform Engineeringの技術トピックをGeminiで選定し、Slackに投稿するCloud Run Jobです。過去14日間の学習履歴を考慮して重複を避け、多様なトピックを提供します。

## 主な機能
- Gemini APIで技術トピックを自動選定（多様性重視）
- 過去14日間のトピック履歴をFirestoreから取得して重複回避
- Slack Block Kitでインタラクティブなメッセージを作成
- 学習履歴をFirestoreに保存

## 環境変数

| 環境変数名 | 必須 | デフォルト値 | 説明 |
|-----------|------|-------------|------|
| `SLACK_BOT_TOKEN` | ✓ | - | Slack Bot User OAuth Token (xoxb-で始まるトークン) |
| `CHANNEL` | ✓ | - | 投稿先Slackチャンネル（例: #ai-sensei） |
| `PROJECT_ID` | ✓ | - | Vertex AI & Firestore用のGoogle Cloud Project ID |
| `LOCATION` | | `global` | Vertex AIのロケーション |
| `GEMINI_MODEL` | | `gemini-3-flash-preview` | 使用するGeminiモデル名 |

## 依存関係

### Google Cloud サービス
- **Vertex AI API**: Geminiによるトピック選定
- **Firestore**: 学習履歴の保存・取得

### 外部サービス
- **Slack**: トピック投稿

### 必要な権限
- `roles/aiplatform.user` - Vertex AI使用権限
- `roles/datastore.user` - Firestore読み書き権限
- Slack Bot Scopes: `chat:write`, `chat:write.public`

## デプロイ方法

### 前提条件
```bash
# Vertex AIを有効化
gcloud services enable aiplatform.googleapis.com

# Firestoreを有効化
gcloud services enable firestore.googleapis.com
```

### デプロイコマンド
```bash
# イメージをビルド
gcloud builds submit --tag gcr.io/your-project/ai-sensei-daily-poster

# Cloud Run Jobとしてデプロイ
gcloud run jobs create ai-sensei-daily-poster \
  --image gcr.io/your-project/ai-sensei-daily-poster \
  --region asia-northeast1 \
  --set-env-vars CHANNEL=#ai-sensei \
  --set-env-vars PROJECT_ID=your-project \
  --set-env-vars LOCATION=global \
  --set-secrets SLACK_BOT_TOKEN=slack-sensei-bot:latest
```

### Cloud Schedulerでの定期実行設定
```bash
# 毎日9:00 JSTに実行
gcloud scheduler jobs create http ai-sensei-daily \
  --location asia-northeast1 \
  --schedule "0 9 * * *" \
  --time-zone "Asia/Tokyo" \
  --uri "https://asia-northeast1-run.googleapis.com/apis/run.googleapis.com/v1/namespaces/your-project/jobs/ai-sensei-daily-poster:run" \
  --http-method POST \
  --oauth-service-account-email your-sa@your-project.iam.gserviceaccount.com
```

## 動作フロー
1. Firestoreから過去14日間のレッスン履歴を取得
2. Gemini APIでトピックを選定（履歴と重複しないよう指示）
3. Slackにボタン付きメッセージを投稿
4. 選定したトピックをFirestoreに保存

## 出力例
```
📚 今日の学習トピック

OIDC (OpenID Connect) の認証フロー

このトピックでは、OIDCの仕組みを深く理解し、認証トークンの検証方法や実装のベストプラクティスを学びます。

[学習を開始] [スキップ]
```

## トピックカテゴリ例
- 認証・認可: OIDC, SAML, mTLS, OAuth 2.0
- コンテナ技術: Linux Namespace, cgroups, OCI
- ネットワーク: DNS, HTTP/3, QUIC, eBPF
- セキュリティ: TLS 1.3, PKI, Vault
- Kubernetes: CNI, Admission Webhook, CRD
- 可観測性: OpenTelemetry, Distributed Tracing
