package config

import "github.com/kelseyhightower/envconfig"

type Config struct {
	SlackWebhookUrl string `envconfig:"SLACK_WEBHOOK" required:"true"`
	Channel         string `envconfig:"CHANNEL" required:"true"`
	BucketName      string `envconfig:"BUCKET_NAME" required:"true"`
	PhotoURL        string `envconfig:"PHOTO_URL" required:"true"`
}

func LoadConfig() (*Config, error) {
	var conf Config
	if err := envconfig.Process("", &conf); err != nil {
		return nil, err
	}
	return &conf, nil
}
