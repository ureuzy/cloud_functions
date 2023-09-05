package auditalert

import (
	"context"
	"fmt"
	"log"
	"os"
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

func main(ctx context.Context, e event.Event) error {
	msg := &MessagePublishedData{}
	if err := e.DataAs(msg); err != nil {
		return fmt.Errorf("event.DataAs: %v", err)
	}
	data := auditdata.LogEntryData{}
	err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(msg.Message.Data, &data)
	if err != nil {
		return fmt.Errorf("got data error %v", err)
	}

	log.Println(data.ProtoPayload.MethodName)

	webhookUrl := os.Getenv("SLACK_WEBHOOK")
	if webhookUrl == "" {
		return fmt.Errorf("must be set slack webhook")
	}
	jst, _ := time.LoadLocation("Asia/Tokyo")
	attachment := slack.Attachment{}
	attachment.
		AddField(slack.Field{Title: "TimeStamp", Value: data.Timestamp.AsTime().In(jst).String()}).
		AddField(slack.Field{Title: "InsertID", Value: data.InsertId}).
		AddField(slack.Field{Title: "PrincipalEmail", Value: data.ProtoPayload.AuthenticationInfo.PrincipalEmail}).
		AddField(slack.Field{Title: "MethodName", Value: data.ProtoPayload.MethodName}).
		AddField(slack.Field{Title: "ResourceName", Value: data.ProtoPayload.ResourceName})
	payload := slack.Payload{
		Username:    "AuditLog",
		Channel:     os.Getenv("CHANNEL"),
		Attachments: []slack.Attachment{attachment},
	}
	errs := slack.Send(webhookUrl, "", payload)
	if len(errs) > 0 {
		fmt.Printf("error: %s\n", errs)
	}
	return nil
}
