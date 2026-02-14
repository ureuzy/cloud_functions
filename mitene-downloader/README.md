# Mitene Downloader

## 概要
みてねの写真・動画を自動的にダウンロードしてGoogle Cloud StorageにバックアップするCloud Run Jobです。新しいメディアのみを差分ダウンロードし、完了時にSlackで通知します。

## 主な機能
- みてねのWebページからメディアファイルURL一覧を取得
- 日付ごとにフォルダ分けしてCloud Storageにアップロード
- 既にバックアップ済みのファイルはスキップ（差分ダウンロード）
- ダウンロード結果をSlackに通知
- ページネーション対応で全メディアを取得

## 環境変数

| 環境変数名 | 必須 | デフォルト値 | 説明 |
|-----------|------|-------------|------|
| `SLACK_WEBHOOK` | ✓ | - | Slack通知用のWebhook URL |
| `CHANNEL` | ✓ | - | 通知先Slackチャンネル |
| `BUCKET_NAME` | ✓ | - | バックアップ先のCloud Storageバケット名 |
| `PHOTO_URL` | ✓ | - | みてねの共有ページURL |

## 依存関係

### Google Cloud サービス
- **Cloud Storage API**: メディアファイルのアップロード

### 外部サービス
- **Slack**: ダウンロード結果の通知
- **みてね**: メディアファイルの取得元

### 必要な権限
- `roles/storage.objectCreator` - Cloud Storageへのオブジェクト作成
- `roles/storage.objectViewer` - 既存オブジェクトの存在確認

## デプロイ方法

### 前提条件
```bash
# Cloud Storage APIを有効化
gcloud services enable storage.googleapis.com

# バックアップ用バケットを作成
gsutil mb -l asia-northeast1 gs://your-backup-bucket
```

### デプロイコマンド
```bash
# イメージをビルド
gcloud builds submit --tag gcr.io/your-project/mitene-downloader

# Cloud Run Jobとしてデプロイ
gcloud run jobs create mitene-downloader \
  --image gcr.io/your-project/mitene-downloader \
  --region asia-northeast1 \
  --set-env-vars BUCKET_NAME=your-backup-bucket \
  --set-env-vars CHANNEL=#photos \
  --set-env-vars SLACK_WEBHOOK=https://hooks.slack.com/services/xxx \
  --set-env-vars PHOTO_URL=https://mitene.us/f/xxxx
```

### Cloud Schedulerでの定期実行設定（推奨）
```bash
# 毎日深夜2:00 JSTに実行
gcloud scheduler jobs create http mitene-downloader-daily \
  --location=asia-northeast1 \
  --schedule="0 2 * * *" \
  --time-zone="Asia/Tokyo" \
  --uri="https://asia-northeast1-run.googleapis.com/apis/run.googleapis.com/v1/namespaces/your-project/jobs/mitene-downloader:run" \
  --http-method=POST \
  --oauth-service-account-email=your-sa@your-project.iam.gserviceaccount.com
```

## 動作フロー
1. みてねの共有ページにアクセス
2. HTMLをパースしてメディアファイルのURL一覧を取得
3. 各ファイルについて:
   - Cloud Storageに同名ファイルが存在するか確認
   - 存在しない場合のみダウンロードしてアップロード
   - 既存ファイルに遭遇したら処理を終了（差分ダウンロード）
4. 結果をSlackに通知

## ストレージ構造
```
gs://your-bucket/
  mitene/
    2024/
      1/
        15/
          1705312800-12345
          1705312800-12346
        16/
          1705399200-12347
```

ファイル名は `{unixタイムスタンプ}-{メディアID}` 形式で保存されます。
