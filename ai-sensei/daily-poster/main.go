package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/slack-go/slack"
	"google.golang.org/genai"

	"github.com/ureuzy/cloud_functions/ai-sensei/daily-poster/config"
)

type TopicSuggestion struct {
	Topic       string `json:"topic"`
	Description string `json:"description"`
}

func main() {
	ctx := context.Background()
	conf, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	log.Printf("Starting AI Sensei Daily Poster")

	// Gemini クライアント初期化
	genaiClient, err := genai.NewClient(ctx, &genai.ClientConfig{
		Project:  conf.ProjectID,
		Location: conf.Location,
		Backend:  genai.BackendVertexAI,
	})
	if err != nil {
		log.Fatalf("Failed to create GenAI client: %v", err)
	}

	// Firestore クライアント初期化
	firestoreClient, err := NewFirestoreClient(ctx, conf.ProjectID)
	if err != nil {
		log.Fatalf("Failed to create Firestore client: %v", err)
	}
	defer firestoreClient.Close()

	// 過去14日分のレッスン履歴を取得
	recentLessons, err := firestoreClient.GetRecentLessons(ctx, 14)
	if err != nil {
		log.Fatalf("Failed to get recent lessons: %v", err)
	}
	log.Printf("Found %d recent lessons", len(recentLessons))

	// Gemini で今日のトピックを選定
	topic, err := selectDailyTopic(ctx, genaiClient, conf.ModelName, recentLessons)
	if err != nil {
		log.Fatalf("Failed to select daily topic: %v", err)
	}

	log.Printf("Selected topic: %s", topic.Topic)

	// Slack に投稿
	slackClient := slack.New(conf.SlackBotToken)
	err = postToSlack(slackClient, conf.Channel, topic)
	if err != nil {
		log.Fatalf("Failed to post to Slack: %v", err)
	}

	// 選定したトピックをFirestoreに保存
	err = firestoreClient.SaveLesson(ctx, topic.Topic, topic.Description)
	if err != nil {
		log.Printf("Warning: Failed to save lesson to Firestore: %v", err)
	}

	log.Println("Daily topic posted successfully!")
}

func selectDailyTopic(ctx context.Context, client *genai.Client, modelName string, recentLessons []LessonHistory) (*TopicSuggestion, error) {
	currentDate := time.Now().Format("2006-01-02")

	// 最近のトピックリストを作成
	var recentTopicsText string
	if len(recentLessons) > 0 {
		recentTopicsText = "\n## 最近行った授業（これらと同じ具体的なトピックは避けてください）\n"
		for _, lesson := range recentLessons {
			recentTopicsText += fmt.Sprintf("- %s (%s)\n", lesson.Topic, lesson.Date.Format("2006-01-02"))
		}
		recentTopicsText += "\n注意: 大枠のカテゴリ（コンテナ技術、ネットワーク、セキュリティなど）が重複するのは問題ありません。具体的なトピックが違えばOKです。\n"
	}

	prompt := fmt.Sprintf(`あなたはSRE/Platform Engineeringの専門家です。
毎日1つ、学習者が深く学ぶべき技術トピックを提案してください。
%s
## 重要: 多様性を重視
- **毎回異なるカテゴリから選定すること**（コンテナ、ネットワーク、認証、CI/CD、可観測性など）
- 同じジャンルが連続しないようにランダムに選ぶこと
- 幅広い技術領域をカバーすること

## 選定基準
- **非常に具体的で深い技術トピック**であること（広範なトピックは避ける）
- SRE/Platform Engineering/インフラエンジニアに実務で役立つこと
- 実践的な演習が可能なこと

## トピック例（多様なカテゴリから1つ選定。あくまで例なので関連しそうな技術であればOKです）
- 認証・認可: 
  - プロトコル: OIDC, SAML, mTLS, OAuth 2.0 Flow, JWT, FIDO2/WebAuthn, SPIFFE/SPIRE
  - 認可モデル: RBAC, ABAC, ReBAC (Relationship-Based), Cedar, Rego (OPA)
  - 実装: Keycloak, Auth0, Zitadel, Ory Kratos/Keto, Dex, Identity Federation

- ネットワーク: 
  - プロトコル: DNS, HTTP/3, QUIC, BGP, VXLAN, Anycast, SR-IOV, DPDK, Segment Routing
  - パケット処理: iptables, NFTables, eBPF XDP, TC (Traffic Control), Maglev Hashing
  - 接続性: Service Mesh (Istio, Linkerd), Ambient Mesh, Tailscale/WireGuard, Ingress Controller

- セキュリティ: 
  - 暗号化: TLS 1.3, AES-GCM, ChaCha20-Poly1305, PKI, KMS (Envelope Encryption)
  - 秘密管理: Vault, External Secrets Operator, Secret Rotation, TPM (Trusted Platform Module)
  - サプライチェーン: SBOM (CycloneDX/SPDX), Sigstore (Cosign), SLSA Framework, Vulnerability Scanning

- コンテナ技術:
  - 分離技術: Linux Namespace, cgroups, Seccomp, AppArmor, LSM (Linux Security Modules)
  - ファイルシステム: OverlayFS, ZFS, btrfs, Snapshot/Clone, Content-addressable storage
  - ランタイム: runc, containerd, CRI-O, Firecracker (MicroVM), gVisor, Wasm/WASI

- プロトコル:
  - 通信形式: gRPC (Streaming), WebSocket, SSE, MQTT, WebTransport, Apache Arrow (Flight)
  - データ構造: Protocol Buffers, Avro, MessagePack, FlatBuffers, CloudEvents
  - APIゲートウェイ: Envoy, Kong, Tyk, Backend for Frontends (BFF)

- 可観測性:
  - テレメトリ: OpenTelemetry, Continuous Profiling (Parca/pyroscope), eBPF Monitoring
  - メトリクス・ログ: Prometheus, Grafana Loki, VictoriaMetrics, ClickHouse (Log Storage)
  - トレース: Distributed Tracing (Jaeger, Tempo), Tail-based Sampling, Service Dependency Maps

- CI/CD:
  - GitOps: ArgoCD Sync, Flux v2, Progressive Delivery (Flagger), Drift Detection
  - パイプライン: Tekton, GitHub Actions, Dagger, CUE (Configuration Language)
  - IaC: Terraform State, Pulumi, Crossplane, CDK8s, KusionStack

- Kubernetes:
  - 内部構造: etcd RAFT, Admission Webhook, Custom Scheduler, Controller Runtime, Client-go
  - 拡張: CRD, Operator Pattern, API Aggregation Layer, Virtual Kubelet, Vcluster
  - 自動化: CNI (Cilium, Calico), CSI, Karpenter, Vertical Pod Autoscaler (VPA)

- ストレージ:
  - 分散ストレージ: Ceph, MinIO, Rook, Longhorn, OpenEBS, HDFS
  - プロトコル/HW: CSI Driver, NVMe-oF, iSCSI, S3 API, POSIX Compliance
  - 理論/構造: LSM-Tree, B-Tree, WAL (Write Ahead Log), Page Cache, Direct I/O

- データベース
  - リレーショナル: PostgreSQL WAL, MySQL Replication, CockroachDB, TiDB, YugabyteDB
  - NoSQL/NewSQL: Redis Cluster, Cassandra, ScyllaDB, MongoDB Sharding, DynamoDB
  - 特殊用途: Vector DB (Milvus, Weaviate), Graph DB (Neo4j), Time Series (InfluxDB)

- メッセージング・イベント駆動
  - ブローカー: Kafka Partition, RabbitMQ Clustering, NATS JetStream, Apache Pulsar, Google Pub/Sub
  - パターン: Event Sourcing, CQRS, Transactional Outbox, Competing Consumers
  - 処理/統合: Kafka Connect, Kafka Streams, Apache Flink, Debezium (CDC)

- AWS (Amazon Web Services)
  - Identity: IAM Role/Policy, IAM Roles Anywhere, Organizations (SCP), Permission Boundary
  - Networking: VPC, Transit Gateway, PrivateLink, Global Accelerator, CloudFront
  - Computing: EC2, Lambda, Fargate, Karpenter (EKS), Nitro System
  - Storage: S3 (Standard/Intelligent-Tiering), EBS, EFS, FSx for Lustre/NetApp ONTAP
  - Governance: Control Tower, Config, CloudTrail, GuardDuty, CloudFormation

- Google Cloud
  - Identity: Cloud IAM, Service Account, Workload Identity Federation, Identity-Aware Proxy (IAP)
  - Networking: Shared VPC, Cloud Armor, Cloud Interconnect, Private Service Connect, Cloud CDN
  - Computing: Compute Engine, Cloud Run, GKE (Autopilot/Standard), Cloud Functions
  - Storage: Cloud Storage, Persistent Disk, Filestore, Hyperdisk
  - Governance: Organization Policy, Resource Manager, Service Usage, Cloud Deployment Manager

- Microsoft Azure
  - Identity: Microsoft Entra ID (Azure AD), Managed Identity, Conditional Access, Azure RBAC
  - Networking: VNet, ExpressRoute, Virtual WAN, Azure Front Door, Private Link
  - Computing: Azure VMs, Azure Container Apps, AKS (Azure Kubernetes Service), Azure Functions
  - Storage: Blob Storage, Azure Files, Managed Disks, Blob Fuse
  - Governance: Azure Policy, Management Groups, Blueprints, Bicep (IaC), Azure Resource Manager (ARM)  

## 出力形式
{
  "topic": "今日のトピック名（日本語）",
  "description": "このトピックで何を学ぶか（2-3行の説明）"
}

今日のトピックを1つ、ランダムに異なるカテゴリから提案してください。

今日の日付: %s`, recentTopicsText, currentDate)

	cfg := &genai.GenerateContentConfig{
		ResponseMIMEType: "application/json",
		Temperature:      genai.Ptr(float32(1.5)), // 高いランダム性
	}

	result, err := client.Models.GenerateContent(ctx, modelName, genai.Text(prompt), cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to generate content: %v", err)
	}

	if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("no content generated")
	}

	var suggestion TopicSuggestion
	err = json.Unmarshal([]byte(result.Candidates[0].Content.Parts[0].Text), &suggestion)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %v, raw: %s", err, result.Candidates[0].Content.Parts[0].Text)
	}

	return &suggestion, nil
}

func postToSlack(slackClient *slack.Client, channel string, topic *TopicSuggestion) error {
	// Slack Block Kit でメッセージを構築
	headerText := fmt.Sprintf("📚 *今日の学習トピック*\n\n*%s*", topic.Topic)
	descriptionText := topic.Description

	blocks := []slack.Block{
		slack.NewSectionBlock(
			slack.NewTextBlockObject("mrkdwn", headerText, false, false),
			nil,
			nil,
		),
		slack.NewSectionBlock(
			slack.NewTextBlockObject("mrkdwn", descriptionText, false, false),
			nil,
			nil,
		),
		slack.NewDividerBlock(),
	}

	// ボタンを追加
	startButton := slack.NewButtonBlockElement(
		"start_learning",
		topic.Topic,
		slack.NewTextBlockObject("plain_text", "学習を開始", true, false),
	)
	startButton.Style = slack.StylePrimary

	skipButton := slack.NewButtonBlockElement(
		"skip_topic",
		topic.Topic,
		slack.NewTextBlockObject("plain_text", "スキップ", true, false),
	)

	actionBlock := slack.NewActionBlock(
		"topic_actions",
		startButton,
		skipButton,
	)

	blocks = append(blocks, actionBlock)

	// メッセージを投稿
	_, _, err := slackClient.PostMessage(
		channel,
		slack.MsgOptionBlocks(blocks...),
		slack.MsgOptionText(fmt.Sprintf("今日のトピック: %s", topic.Topic), false),
	)

	if err != nil {
		return fmt.Errorf("failed to post message: %v", err)
	}

	return nil
}
