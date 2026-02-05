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
	jst, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		return "", err
	}
	return l.Timestamp.AsTime().In(jst).Format("2006/01/02 15:04:05"), nil
}

func (l *LogEntry) getPrincipalEmail() string {
	return l.ProtoPayload.AuthenticationInfo.PrincipalEmail
}

func (l *LogEntry) getColor() string {
	switch l.getPartialMethodName() {
	case "InsertJob":
		return "#36a64f"
	case "SetIamPolicy":
		return "#d3381c"
	default:
		return ""
	}
}

func run(ctx context.Context, e event.Event) error {

	conf, err := config.LoadConfig()
	if err != nil {
		return err
	}

	logEntry, err := eventToLogEntry(e)
	if err != nil {
		return err
	}

	t, err := logEntry.getTime()
	if err != nil {
		return err
	}

	timestamp := slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*TimeStamp*\n %s", t), false, false)
	principalEmail := slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*PrincipalEmail*\n %s", logEntry.getPrincipalEmail()), false, false)
	methodName := slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*MethodName*\n %s", logEntry.getPartialMethodName()), false, false)
	targetProject := slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*TargetProject*\n %s", logEntry.getTargetProject()), false, false)
	viewLog := slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("<%s|ViewLog>", logEntry.buildSelfLink(conf.StorageScope, conf.Project)), false, false)

	fieldsSection := slack.NewSectionBlock(nil, []*slack.TextBlockObject{
		timestamp,
		principalEmail,
		methodName,
		targetProject,
	}, nil)
	linkSection := slack.NewSectionBlock(nil, []*slack.TextBlockObject{
		viewLog,
	}, nil)

	blocks := slack.NewBlockMessage(fieldsSection, slack.NewDividerBlock(), linkSection).Blocks
	attachment := slack.Attachment{
		Color:  logEntry.getColor(),
		Blocks: blocks,
	}

	err = slack.PostWebhook(conf.SlackWebhookUrl, &slack.WebhookMessage{
		Username:    "AuditLog",
		Channel:     conf.Channel,
		Attachments: []slack.Attachment{attachment},
	})
	if err != nil {
		log.Println(err)
		return err
	}
	return nil
}

