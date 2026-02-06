package config

import "github.com/kelseyhightower/envconfig"

type Config struct {
	SlackWebhookUrl string   `envconfig:"SLACK_WEBHOOK" required:"true"`
	Channel         string   `envconfig:"CHANNEL" required:"true"`
	TargetURLs      []string `envconfig:"TARGET_URLS" required:"true"`
	ProjectID       string   `envconfig:"PROJECT_ID" required:"true"`
	Location        string   `envconfig:"LOCATION" default:"us-central1"`
	ModelName       string   `envconfig:"GEMINI_MODEL" default:"gemini-1.5-flash"`
}

func LoadConfig() (*Config, error) {
	var conf Config
	if err := envconfig.Process("", &conf); err != nil {
		return nil, err
	}
	return &conf, nil
}
