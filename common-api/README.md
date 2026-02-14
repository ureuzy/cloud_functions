# Common API

## 概要
Google CloudリソースをWebフロントエンドから管理するためのRESTful APIサーバーです。Firebase Authenticationによる認証、Cloud Schedulerジョブの管理、Cloud Storageバケットの情報取得を提供するCloud Run Serviceです。

## 主な機能
- Firebase Authenticationによるリクエスト認証
- Cloud Schedulerジョブの一覧・一時停止・再開・手動実行
- Cloud Storageバケットの属性取得・オブジェクトカウント
- CORSミドルウェアによるクロスオリジンリクエスト対応
- ヘルスチェックエンドポイント

## 環境変数

| 環境変数名 | 必須 | デフォルト値 | 説明 |
|-----------|------|-------------|------|
| `PORT` | | `8080` | サーバーのポート番号 |
| `BUCKET_NAME` | ✓ | - | 管理対象のCloud Storageバケット名 |
| `PROJECT_ID` | ✓ | - | Google Cloud Project ID |
| `FIREBASE_PROJECT_ID` | ✓ | - | Firebase認証用のProject ID |

## 依存関係

### Google Cloud サービス
- **Cloud Scheduler API**: ジョブの管理
- **Cloud Storage API**: バケット情報の取得
- **Firebase Authentication**: リクエスト認証

### 必要な権限
- `roles/cloudscheduler.admin` - Schedulerジョブの管理
- `roles/storage.objectViewer` - Storageオブジェクトの読み取り
- `roles/firebase.sdkAdminServiceAgent` - Firebase認証

## デプロイ方法

### 前提条件
```bash
# Cloud Scheduler APIを有効化
gcloud services enable cloudscheduler.googleapis.com

# Cloud Storage APIを有効化
gcloud services enable storage.googleapis.com

# Firebase Authenticationが設定済みであること
```

### デプロイコマンド
```bash
# イメージをビルド
gcloud builds submit --tag gcr.io/your-project/common-api

# Cloud Run Serviceとしてデプロイ
gcloud run deploy common-api \
  --image gcr.io/your-project/common-api \
  --region asia-northeast1 \
  --set-env-vars PROJECT_ID=your-project \
  --set-env-vars FIREBASE_PROJECT_ID=your-firebase-project \
  --set-env-vars BUCKET_NAME=your-bucket \
  --allow-unauthenticated
```

## エンドポイント

| パス | メソッド | 認証 | 説明 |
|------|---------|------|------|
| `/healthz` | GET | 不要 | ヘルスチェック |
| `/` | GET | 不要 | APIステータス確認 |
| `/api/v1/scheduler/jobs` | GET | 必要 | Schedulerジョブ一覧 |
| `/api/v1/scheduler/pause` | POST | 必要 | ジョブの一時停止 |
| `/api/v1/scheduler/resume` | POST | 必要 | ジョブの再開 |
| `/api/v1/scheduler/run` | POST | 必要 | ジョブの手動実行 |
| `/api/v1/storage/bucket` | GET | 必要 | バケット属性取得 |
| `/api/v1/storage/count` | GET | 必要 | オブジェクト数取得 |

## 認証
`/api/v1/*` 配下のエンドポイントはFirebase Authenticationによる認証が必要です。リクエストヘッダーに `Authorization: Bearer <Firebase ID Token>` を含めてください。
