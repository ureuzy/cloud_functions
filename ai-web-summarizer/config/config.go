package config

import "github.com/kelseyhightower/envconfig"

type Config struct {
	SlackBotToken   string   `envconfig:"SLACK_BOT_TOKEN" required:"true"`
	Channel         string   `envconfig:"CHANNEL" required:"true"`
	TargetURLs      []string `envconfig:"TARGET_URLS" required:"true"`
	ProjectID       string   `envconfig:"PROJECT_ID" required:"true"`
	Location        string   `envconfig:"LOCATION" default:"global"`
	ModelName       string   `envconfig:"GEMINI_MODEL" default:"gemini-3-flash-preview"`
}

func LoadConfig() (*Config, error) {
	var conf Config
	if err := envconfig.Process("", &conf); err != nil {
		return nil, err
	}
	return &conf, nil
}
