# Activity Analyzer

## 概要
Google Cloud Projectsの全サービスアカウントのアクティビティを分析し、未使用のサービスアカウントを検出してSlackに通知するCloud Functionです。

## 主な機能
- 全Google Cloud Projectsのサービスアカウント利用状況を取得
- 未使用のユーザー作成サービスアカウントを検出
- 作成後一定期間経過したサービスアカウントのみをフィルタリング
- Policy Analyzer APIを使用して認証履歴を分析
- 検出結果をSlackに通知

## 環境変数

| 環境変数名 | 必須 | デフォルト値 | 説明 |
|-----------|------|-------------|------|
| `SLACK_WEBHOOK` | ✓ | - | Slack通知用のWebhook URL |
| `CHANNEL` | ✓ | - | 通知先Slackチャンネル |
| `DAYS_AFTER_CREATION` | | `7` | サービスアカウント作成後の経過日数（この日数を超えたものを対象とする） |

## 依存関係

### Google Cloud サービス
- **Policy Analyzer API**: サービスアカウントの認証履歴を取得
- **Resource Manager API**: プロジェクト一覧を取得

### 外部サービス
- **Slack**: 検出結果の通知

### 必要な権限
- `roles/policyanalyzer.activityAnalyzer` - Policy Analyzerへのアクセス
- `roles/resourcemanager.organizationViewer` - プロジェクト一覧の取得

## デプロイ方法

### 前提条件
```bash
# Policy Analyzer APIを有効化
gcloud services enable policyanalyzer.googleapis.com

# Resource Manager APIを有効化
gcloud services enable cloudresourcemanager.googleapis.com
```

### デプロイコマンド
```bash
gcloud functions deploy activity-analyzer \
  --gen2 \
  --region=asia-northeast1 \
  --runtime=go122 \
  --source=. \
  --entry-point=main \
  --trigger-http \
  --allow-unauthenticated \
  --set-env-vars SLACK_WEBHOOK=https://hooks.slack.com/services/xxx \
  --set-env-vars CHANNEL=#your-channel \
  --set-env-vars DAYS_AFTER_CREATION=7
```

### Cloud Schedulerでの定期実行設定（推奨）
```bash
# 毎週月曜日の10:00 JSTに実行
gcloud scheduler jobs create http activity-analyzer-weekly \
  --location=asia-northeast1 \
  --schedule="0 10 * * 1" \
  --time-zone="Asia/Tokyo" \
  --uri="https://asia-northeast1-your-project.cloudfunctions.net/activity-analyzer" \
  --http-method=GET
```

## 動作フロー
1. Resource Manager APIで全プロジェクトを取得
2. 各プロジェクトのPolicy Analyzer APIでサービスアカウント認証履歴を取得
3. 以下の条件でフィルタリング:
   - 未使用のサービスアカウント（認証履歴なし）
   - ユーザーが作成したサービスアカウント（デフォルトのSAを除外）
   - 作成後指定日数以上経過したサービスアカウント
4. 検出結果をSlackに送信

## 出力例
```
Unused service accounts (More than 7 days after creation)
2024-01-01T00:00:00Z ~ : my-service-account@project-id.iam.gserviceaccount.com
2024-01-05T00:00:00Z ~ : another-account@project-id.iam.gserviceaccount.com
```
