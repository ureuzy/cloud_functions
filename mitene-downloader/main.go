package mitene_downloader

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"time"

	"cloud.google.com/go/storage"
	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/dop251/goja"
	"github.com/go-resty/resty/v2"
	"github.com/slack-go/slack"
	"golang.org/x/net/html"

	"github.com/ureuzy/cloud_functions/mitene-downloader/config"
)

func init() {
	functions.CloudEvent("Main", main)
}

func main(ctx context.Context, e event.Event) error {
	conf, err := config.LoadConfig()
	if err != nil {
		return err
	}

	client := resty.New()
	page := 1
	var result []string
	defer func() {
		if len(result) == 0 {
			return
		}
		toSlack(conf, result)
	}()
out:
	for {
		resp, err := client.R().Get(fmt.Sprintf("%s?page=%d", conf.PhotoURL, page))
		if err != nil {
			log.Println(err.Error())
		}

		links, err := ParseLinks(resp.Body())
		if err != nil {
			log.Println(err.Error())
		}

		var files MediaFiles
		if err = json.Unmarshal([]byte(links), &files); err != nil {
			log.Println(err)
		}

		storageClient, err := storage.NewClient(ctx)
		if err != nil {
			log.Fatal(err)
		}
		gs := WrapperStorageClient{
			storageClient,
			conf.BucketName,
		}

		if len(files) == 0 {
			break
		}

		fmt.Printf("Page Number: %d\n", page)
		fmt.Printf("Page Size: %d\n", len(files))

		for _, file := range files {
			path := buildPath(file)
			if gs.exists(ctx, path) {
				fmt.Printf("%s: already exists\n", path)
				break out
			}
			writer := gs.Bucket(conf.BucketName).Object(path).NewWriter(ctx)
			res, err := client.R().Get(file.ExpiringURL)
			if err != nil {
				log.Println(err.Error())
				continue
			}
			if _, err = io.Copy(writer, bytes.NewReader(res.Body())); err != nil {
				log.Println(err.Error())
				continue
			}

			if err = writer.Close(); err != nil {
				log.Println(err.Error())
				continue
			}
			log.Printf("Downloaded: %d - %s\n", file.ID, file.TookAt.String())
			result = append(result, path)
			time.Sleep(time.Second * 1)
		}
		page += 1
	}
	return nil
}

func buildPath(file *MediaFile) string {
	return fmt.Sprintf("mitene/%d/%d/%d/%s",
		file.TookAt.Year(),
		int(file.TookAt.Month()),
		file.TookAt.Day(),
		fmt.Sprintf("%d-%d", file.TookAt.Unix(), file.ID),
	)
}

func ParseLinks(body []byte) (string, error) {
	node, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		log.Println(err.Error())
	}

	script := node.
		FirstChild.
		NextSibling.
		FirstChild.
		FirstChild.
		NextSibling.
		NextSibling.
		NextSibling.
		NextSibling.
		NextSibling.
		NextSibling.
		NextSibling.
		NextSibling.
		FirstChild.
		Data

	vm := goja.New()
	result, err := vm.RunString(`window = {}; gon = {};` + script + `JSON.stringify(gon.media.mediaFiles)`)
	if err != nil {
		return "", err
	}
	return result.String(), nil
}

func toSlack(conf *config.Config, result []string) {
	var files string
	for _, i := range result {
		files += fmt.Sprintf("%s\n", i)
	}
	attachment := slack.Attachment{
		Title: fmt.Sprintf("%d file uploaded", len(result)),
		Text:  files,
	}
	if err := slack.PostWebhook(conf.SlackWebhookUrl, &slack.WebhookMessage{
		Username:    "Mitene Uploader",
		Channel:     conf.Channel,
		Attachments: []slack.Attachment{attachment},
	}); err != nil {
		log.Println(err)
	}
}
