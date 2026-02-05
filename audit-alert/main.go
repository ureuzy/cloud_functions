package main

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/GoogleCloudPlatform/functions-framework-go/funcframework"
	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/googleapis/google-cloudevents-go/cloud/auditdata"
	"github.com/slack-go/slack"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/ureuzy/cloud_functions/audit-alert/config"
)

func init() {
	functions.CloudEvent("Main", run)
}

func main() {
	// Use PORT environment variable, or default to 8080.
	port := "8080"
	if envPort := os.Getenv("PORT"); envPort != "" {
		port = envPort
	}
	if err := funcframework.Start(port); err != nil {
		log.Fatalf("funcframework.Start: %v\n", err)
	}
}

type MessagePublishedData struct {
	Message PubSubMessage
}

type PubSubMessage struct {
	Data []byte `json:"data"`
}

type LogEntry struct {
	*auditdata.LogEntryData
}

func eventToLogEntry(e event.Event) (*LogEntry, error) {
	msg := &MessagePublishedData{}
	if err := e.DataAs(msg); err != nil {
		return nil, fmt.Errorf("event.DataAs: %v", err)
	}
	data := auditdata.LogEntryData{}
	err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(msg.Message.Data, &data)
	if err != nil {
		return nil, fmt.Errorf("got data error %v", err)
	}
	return &LogEntry{&data}, nil
}

func (l *LogEntry) buildSelfLink(storageScope string, projectId string) string {
	query := fmt.Sprintf("insertId=\"%s\";storageScope=storage,%s", l.InsertId, url.PathEscape(storageScope))
	u := url.URL{
		Scheme:   "https",
		Host:     "console.cloud.google.com",
		Path:     path.Join("logs", fmt.Sprintf("query;query=%s", query)),
		RawQuery: fmt.Sprintf("project=%s", projectId),
	}
	return strings.Replace(u.String(), "%252F", "%2F", -1)
}

func (l *LogEntry) getTargetProject() string {
	return l.Resource.Labels["project_id"]
}

func (l *LogEntry) getPartialMethodName() string {
	s := strings.Split(l.ProtoPayload.MethodName, ".")
	return s[len(s)-1]
}

func (l *LogEntry) getTime() (string, error) {
	jst := time.FixedZone("Asia/Tokyo", 9*60*60)
	if l.Timestamp == nil {
		return time.Now().In(jst).Format("2006/01/02 15:04:05"), nil
	}
	return l.Timestamp.AsTime().In(jst).Format("2006/01/02 15:04:05"), nil
}

func (l *LogEntry) getPrincipalEmail() string {
	if l.ProtoPayload == nil || l.ProtoPayload.AuthenticationInfo == nil {
		return "unknown"
	}
	return l.ProtoPayload.AuthenticationInfo.PrincipalEmail
}

func (l *LogEntry) getColor() string {
	if l.ProtoPayload == nil {
		return ""
	}
	switch l.getPartialMethodName() {
	case "InsertJob":
		return "#36a64f"
	case "SetIamPolicy":
		return "#d3381c"
	default:
		return ""
	}
}

func (l *LogEntry) getIAMDeltas() string {
	if l.ProtoPayload == nil || l.ProtoPayload.Metadata == nil {
		return ""
	}

	// metadata.bindingDeltas を探す
	deltas, ok := l.ProtoPayload.Metadata.AsMap()["bindingDeltas"].([]interface{})
	if !ok || len(deltas) == 0 {
		return ""
	}

	var result strings.Builder
	for _, d := range deltas {
		delta, ok := d.(map[string]interface{})
		if !ok {
			continue
		}
		action := delta["action"] // ADD or REMOVE
		role := delta["role"]
		member := delta["member"]
		result.WriteString(fmt.Sprintf("• *%s*: %s to %s\n", action, role, member))
	}
	return result.String()
}

func run(ctx context.Context, e event.Event) error {
	log.Printf("Processing event ID: %s, Type: %s", e.ID(), e.Type())

	conf, err := config.LoadConfig()
	if err != nil {
		log.Printf("Error loading config: %v", err)
		return err
	}

	logEntry, err := eventToLogEntry(e)
	if err != nil {
		log.Printf("Error converting event to log entry: %v", err)
		return err
	}

	t, err := logEntry.getTime()
	if err != nil {
		log.Printf("Error getting time: %v", err)
		return err
	}

	principalEmail := logEntry.getPrincipalEmail()
	methodName := logEntry.getPartialMethodName()
	log.Printf("Audit Log: Method=%s, User=%s", methodName, principalEmail)

	// 基本情報の組み立て
	timestampObj := slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*TimeStamp*\n %s", t), false, false)
	principalEmailObj := slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*PrincipalEmail*\n %s", principalEmail), false, false)
	methodNameObj := slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*MethodName*\n %s", methodName), false, false)
	targetProjectObj := slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*TargetProject*\n %s", logEntry.getTargetProject()), false, false)

	fields := []*slack.TextBlockObject{
		timestampObj,
		principalEmailObj,
		methodNameObj,
		targetProjectObj,
	}

	blocks := []slack.Block{
		slack.NewSectionBlock(nil, fields, nil),
	}

	// IAM詳細情報があれば追加
	if iamDeltas := logEntry.getIAMDeltas(); iamDeltas != "" {
		iamHeader := slack.NewContextBlock("", slack.NewTextBlockObject("mrkdwn", "*IAM Policy Changes*", false, false))
		iamContent := slack.NewSectionBlock(slack.NewTextBlockObject("mrkdwn", iamDeltas, false, false), nil, nil)
		blocks = append(blocks, slack.NewDividerBlock(), iamHeader, iamContent)
	}

	// ログへのリンク追加
	viewLogObj := slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("<%s|ViewLog>", logEntry.buildSelfLink(conf.StorageScope, conf.Project)), false, false)
	blocks = append(blocks, slack.NewDividerBlock(), slack.NewSectionBlock(nil, []*slack.TextBlockObject{viewLogObj}, nil))

	attachment := slack.Attachment{
		Color:  logEntry.getColor(),
		Blocks: slack.Blocks{BlockSet: blocks},
	}

	log.Printf("Sending notification to Slack channel: %s", conf.Channel)
	err = slack.PostWebhook(conf.SlackWebhookUrl, &slack.WebhookMessage{
		Username:    "AuditLog",
		Channel:     conf.Channel,
		Attachments: []slack.Attachment{attachment},
	})
	if err != nil {
		log.Printf("Error posting to Slack: %v", err)
		return err
	}

	log.Printf("Successfully sent notification for event %s", e.ID())
	return nil
}

