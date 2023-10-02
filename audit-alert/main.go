package auditalert

import (
	"context"
	"errors"
	"fmt"
	"github.com/ureuzy/cloud_functions/audit-alert/config"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/ashwanthkumar/slack-go-webhook"
	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/googleapis/google-cloudevents-go/cloud/auditdata"
	"google.golang.org/protobuf/encoding/protojson"
)

func init() {
	functions.CloudEvent("Main", main)
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
		return "good"
	case "SetIamPolicy":
		return "danger"
	default:
		return ""
	}
}

func toPtr(s string) *string {
	return &s
}

func main(ctx context.Context, e event.Event) error {

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

	attachment := slack.Attachment{}
	attachment.
		AddField(slack.Field{Title: "TimeStamp", Value: t}).
		AddField(slack.Field{Title: "PrincipalEmail", Value: logEntry.getPrincipalEmail()}).
		AddField(slack.Field{Title: "MethodName", Value: logEntry.getPartialMethodName()}).
		AddField(slack.Field{Title: "TargetProject", Value: logEntry.getTargetProject()}).
		AddAction(slack.Action{
			Text:  "ViewLog",
			Url:   logEntry.buildSelfLink(conf.StorageScope, conf.Project),
			Style: "",
		})
	attachment.Color = toPtr(logEntry.getColor())
	payload := slack.Payload{
		Username:    "AuditLog",
		Channel:     os.Getenv("CHANNEL"),
		Attachments: []slack.Attachment{attachment},
	}
	errs := slack.Send(conf.SlackWebhookUrl, "", payload)
	if len(errs) > 0 {
		return errors.New(fmt.Sprintf("error: %s\n", errs))
	}
	return nil
}
