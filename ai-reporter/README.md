# AI Reporter

## 概要
Google CloudのリリースノートをBigQueryから取得し、Gemini APIで要約してSlackに投稿するCloud Functionです。製品ごとに整理され、スレッド形式で詳細が提供されます。

## 主な機能
- BigQueryのパブリックデータセットからGoogle Cloudリリースノートを取得
- Gemini APIを使用してリリースノートを日本語で要約
- 製品ごとにグループ化して整理
- Slackにメインスレッドとサブスレッドで階層的に投稿
- レート制限対応（429エラーのリトライ処理）

## 環境変数

| 環境変数名 | 必須 | デフォルト値 | 説明 |
|-----------|------|-------------|------|
| `SLACK_BOT_TOKEN` | ✓ | - | Slack Bot User OAuth Token (xoxb-で始まるトークン) |
| `CHANNEL` | ✓ | - | 投稿先Slackチャンネル（例: #release-notes） |
| `PROJECT_ID` | ✓ | - | Vertex AI用のGoogle Cloud Project ID |
| `BIGQUERY_PROJECT_ID` | ✓ | - | BigQuery用のGoogle Cloud Project ID |
| `LOCATION` | | `global` | Vertex AIのロケーション |
| `GEMINI_MODEL` | | `gemini-3-flash-preview` | 使用するGeminiモデル名 |
| `DAYS_AGO` | | `1` | 何日前のリリースノートを取得するか |

## 依存関係

### Google Cloud サービス
- **BigQuery API**: リリースノートデータの取得（`bigquery-public-data.google_cloud_release_notes.release_notes`）
- **Vertex AI API**: Gemini APIによる要約生成

### 外部サービス
- **Slack**: リリースノート投稿

### 必要な権限
- `roles/aiplatform.user` - Vertex AI使用権限
- `roles/bigquery.jobUser` - BigQueryクエリ実行権限
- Slack Bot Scopes: `chat:write`, `chat:write.public`

## デプロイ方法

### 前提条件
```bash
# Vertex AIを有効化
gcloud services enable aiplatform.googleapis.com

# BigQuery APIを有効化
gcloud services enable bigquery.googleapis.com
```

### デプロイコマンド
```bash
gcloud functions deploy ai-reporter \
  --gen2 \
  --region=asia-northeast1 \
  --runtime=go122 \
  --source=. \
  --entry-point=main \
  --trigger-http \
  --allow-unauthenticated \
  --set-env-vars PROJECT_ID=your-vertex-ai-project \
  --set-env-vars BIGQUERY_PROJECT_ID=your-bigquery-project \
  --set-env-vars CHANNEL=#release-notes \
  --set-env-vars DAYS_AGO=1 \
  --set-secrets SLACK_BOT_TOKEN=slack-bot-token:latest
```

### Cloud Schedulerでの定期実行設定（推奨）
```bash
# 毎日10:00 JSTに前日のリリースノートを取得
gcloud scheduler jobs create http ai-reporter-daily \
  --location=asia-northeast1 \
  --schedule="0 10 * * *" \
  --time-zone="Asia/Tokyo" \
  --uri="https://asia-northeast1-your-project.cloudfunctions.net/ai-reporter" \
  --http-method=GET
```

## 動作フロー
1. 指定された日付のリリースノートをBigQueryから取得
2. 製品（ProductName）ごとにグループ化
3. 各製品のリリースノートをGemini APIで要約
4. メインスレッドにサマリーを投稿
5. 各製品の詳細をスレッドに投稿

## 出力形式

### メインスレッド
```
Google Cloud Release Notes
2024/01/15 のリリースノートは 25 件です（8 製品）。
```

### サブスレッド（製品ごと）
```
📦 Cloud Run (アップデート: 3 件)
  1. 新しいCPU制限機能の追加
  2. メモリ最適化の改善
  3. 起動時間の高速化

詳細:
• 新しいCPU制限機能により、コストを削減できます
• メモリ使用量が平均30%削減されました
• コールドスタート時間が50%短縮されました
```
