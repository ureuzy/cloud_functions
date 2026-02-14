# Cloud Functions

Google Cloud上で動作する各種自動化ツール・Botのモノレポです。Go言語で実装され、Cloud Run / Cloud Functionsとしてデプロイされます。

## Functions一覧

| Function | 種別 | 説明 |
|----------|------|------|
| [activity-analyzer](./activity-analyzer/) | Cloud Function | 未使用サービスアカウントの検出・Slack通知 |
| [ai-reporter](./ai-reporter/) | Cloud Run Job | Google Cloudリリースノートの要約・Slack投稿 |
| [ai-sensei](./ai-sensei/) | Cloud Run Job / Service | AI対話型技術学習Bot（Daily Poster + Event Handler） |
| [audit-alert](./audit-alert/) | Cloud Function | Audit Logの監視・Slack通知（Pub/Subトリガー） |
| [billing-monitor](./billing-monitor/) | Cloud Run Job | 請求コストの可視化・Slack通知 |
| [common-api](./common-api/) | Cloud Run Service | GCPリソース管理用REST API（Firebase認証付き） |
| [mitene-downloader](./mitene-downloader/) | Cloud Run Job | みてね写真・動画のCloud Storageバックアップ |

## ディレクトリ構造

```
cloud_functions/
├── activity-analyzer/     # 未使用サービスアカウント検出
│   ├── main.go
│   └── config/
├── ai-reporter/           # リリースノート要約Bot
│   ├── main.go
│   └── config/
├── ai-sensei/             # AI技術学習Bot
│   ├── daily-poster/      #   毎日のトピック投稿
│   │   ├── main.go
│   │   └── config/
│   └── event-handler/     #   Slack対話処理
│       ├── main.go
│       └── config/
├── audit-alert/           # 監査ログ通知
│   ├── main.go
│   └── config/
├── billing-monitor/       # 請求コスト通知
│   ├── main.go
│   └── config/
├── common-api/            # GCPリソース管理API
│   ├── main.go
│   └── config/
└── mitene-downloader/     # みてねバックアップ
    ├── main.go
    └── config/
```

## 共通セットアップ

### 前提条件
- Go 1.22+
- Google Cloud SDK (`gcloud`)
- Google Cloud Project（各functionに必要なAPIを有効化済み）

### ローカル開発
```bash
# 依存関係のインストール
cd <function-name>
go mod download

# ローカル実行
go run .
```

### 環境変数の管理
各functionの環境変数は `config/config.go` に定義されています。`envconfig` ライブラリを使用しており、環境変数から自動的に読み込まれます。

## 共通の外部サービス

- **Slack**: ほぼ全てのfunctionで通知・投稿に使用
- **BigQuery**: データ取得（ai-reporter, billing-monitor）
- **Firestore**: データ永続化（ai-sensei）
- **Vertex AI (Gemini)**: AI処理（ai-reporter, ai-sensei）
- **Cloud Storage**: ファイル管理（mitene-downloader, common-api）
