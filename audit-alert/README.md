# Audit Alert

## 概要
Google CloudのAudit Logをリアルタイムで監視し、重要な操作（IAMポリシー変更など）をSlackに通知するCloud Functionです。Cloud Pub/Subトリガーで動作します。

## 主な機能
- Cloud Audit LogsのPub/Subメッセージを処理
- IAMポリシー変更の詳細を抽出（bindingDeltas）
- 操作者・対象プロジェクト・実行時刻などの情報を整形
- Google Cloud Consoleのログ詳細へのリンクを生成
- Slackに構造化されたメッセージを通知

## 環境変数

| 環境変数名 | 必須 | デフォルト値 | 説明 |
|-----------|------|-------------|------|
| `SLACK_WEBHOOK` | ✓ | - | Slack通知用のWebhook URL |
| `CHANNEL` | ✓ | - | 通知先Slackチャンネル |
| `STORAGE_SCOPE` | ✓ | - | ログのStorageスコープ（例: `projects/PROJECT_ID`） |
| `PROJECT` | ✓ | - | 監視対象のGoogle Cloud Project ID |

## 依存関係

### Google Cloud サービス
- **Cloud Audit Logs**: 監査ログの取得
- **Cloud Pub/Sub**: ログイベントの配信

### 外部サービス
- **Slack**: 監査ログ通知

### 必要な権限
- Pub/Subトリガーで自動実行されるため、特別な権限は不要

## デプロイ方法

### 前提条件
```bash
# Audit LogsをPub/Subにエクスポートするログシンクを作成
gcloud logging sinks create audit-log-sink \
  pubsub.googleapis.com/projects/your-project/topics/audit-logs \
  --log-filter='protoPayload.methodName="SetIamPolicy"
    OR protoPayload.methodName="DeleteServiceAccount"
    OR protoPayload.methodName="CreateServiceAccount"'
```

### デプロイコマンド
```bash
gcloud functions deploy audit-alert \
  --gen2 \
  --region=asia-northeast1 \
  --runtime=go122 \
  --source=. \
  --entry-point=Main \
  --trigger-topic=audit-logs \
  --set-env-vars SLACK_WEBHOOK=https://hooks.slack.com/services/xxx \
  --set-env-vars CHANNEL=#security-alerts \
  --set-env-vars STORAGE_SCOPE=projects/your-project \
  --set-env-vars PROJECT=your-project
```

## 監視対象の操作例
- IAMポリシーの変更（`SetIamPolicy`）
- サービスアカウントの作成（`CreateServiceAccount`）
- サービスアカウントの削除（`DeleteServiceAccount`）
- その他のAudit Log操作

## 出力例
```
TimeStamp: 2024/01/15 14:30:00
PrincipalEmail: user@example.com
MethodName: SetIamPolicy
TargetProject: your-project-id

---
IAM Policy Changes
• ADD: roles/editor to user:new-user@example.com
• REMOVE: roles/viewer from serviceAccount:old-sa@project.iam.gserviceaccount.com

---
[ViewLog] (クリックでGoogle Cloud Consoleのログ詳細へ)
```

## ログフィルタのカスタマイズ
監視したい操作を追加・変更する場合は、ログシンクのフィルタを編集してください。

```bash
gcloud logging sinks update audit-log-sink \
  --log-filter='protoPayload.methodName="SetIamPolicy"
    OR protoPayload.methodName="DeleteServiceAccount"
    OR protoPayload.methodName="CreateServiceAccount"
    OR protoPayload.methodName="storage.objects.create"'
```

## トラブルシューティング

### 通知が来ない場合
1. ログシンクが正しく設定されているか確認:
   ```bash
   gcloud logging sinks describe audit-log-sink
   ```
2. Pub/Subトピックにメッセージが届いているか確認:
   ```bash
   gcloud pubsub topics list
   gcloud pubsub subscriptions list --topic=audit-logs
   ```
3. Cloud Functionのログを確認:
   ```bash
   gcloud functions logs read audit-alert --region=asia-northeast1
   ```
