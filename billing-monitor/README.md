# Billing Monitor

## 概要
Google Cloudの請求データをBigQueryから取得し、当月の累計コストと直近7日間の推移をSlackに通知するCloud Run Jobです。サービス別コストの内訳もスレッドで提供します。

## 主な機能
- BigQueryから当月の請求データを取得
- 日別コストの推移をプログレスバー付きで表示
- 当月累計コストを計算
- サービス別コスト内訳（上位10件）をスレッドに投稿
- Slack Block Kitで見やすいフォーマットに整形

## 環境変数

| 環境変数名 | 必須 | デフォルト値 | 説明 |
|-----------|------|-------------|------|
| `SLACK_BOT_TOKEN` | ✓ | - | Slack Bot User OAuth Token (xoxb-で始まるトークン) |
| `CHANNEL` | ✓ | - | 投稿先Slackチャンネル |
| `PROJECT_ID` | ✓ | - | Google Cloud Project ID |
| `BILLING_TABLE` | ✓ | - | BigQuery請求テーブル名（例: `project.dataset.table`） |

## 依存関係

### Google Cloud サービス
- **BigQuery API**: 請求データの取得

### 外部サービス
- **Slack**: コストレポートの投稿

### 必要な権限
- `roles/bigquery.jobUser` - BigQueryクエリ実行権限
- `roles/bigquery.dataViewer` - 請求テーブルの読み取り権限
- Slack Bot Scopes: `chat:write`, `chat:write.public`

## デプロイ方法

### 前提条件
```bash
# BigQuery APIを有効化
gcloud services enable bigquery.googleapis.com

# 請求データのBigQueryエクスポートが設定済みであること
```

### デプロイコマンド
```bash
# イメージをビルド
gcloud builds submit --tag gcr.io/your-project/billing-monitor

# Cloud Run Jobとしてデプロイ
gcloud run jobs create billing-monitor \
  --image gcr.io/your-project/billing-monitor \
  --region asia-northeast1 \
  --set-env-vars PROJECT_ID=your-project \
  --set-env-vars BILLING_TABLE=your-project.billing_dataset.gcp_billing_export \
  --set-env-vars CHANNEL=#billing \
  --set-secrets SLACK_BOT_TOKEN=slack-bot-token:latest
```

### Cloud Schedulerでの定期実行設定（推奨）
```bash
# 毎日9:00 JSTに実行
gcloud scheduler jobs create http billing-monitor-daily \
  --location=asia-northeast1 \
  --schedule="0 9 * * *" \
  --time-zone="Asia/Tokyo" \
  --uri="https://asia-northeast1-run.googleapis.com/apis/run.googleapis.com/v1/namespaces/your-project/jobs/billing-monitor:run" \
  --http-method=POST \
  --oauth-service-account-email=your-sa@your-project.iam.gserviceaccount.com
```

## 動作フロー
1. BigQueryから当月1日以降の日別コストデータを取得
2. サービス別コストデータを取得
3. 直近7日間のプログレスバーチャートを生成
4. メインスレッドに累計コストと日別推移を投稿
5. サブスレッドにサービス別コスト（上位10件）を投稿

## 出力例

### メインメッセージ
```
💸 Google Cloud Cost Report
──────────────────────────
• 当月累計: ¥12345.67

直近7日間の推移
02-08 ████████░░░░░░░ ¥  1234
02-07 ██████░░░░░░░░░ ¥   987
02-06 ███████████░░░░ ¥  1567
...
```

### スレッド（サービス別）
```
サービス別コスト（当月累計・上位10件）

1. Cloud Run: ¥3456.78
2. Cloud Storage: ¥2345.67
3. Compute Engine: ¥1234.56
...
```
