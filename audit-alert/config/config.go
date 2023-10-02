package config

import "github.com/kelseyhightower/envconfig"

type Config struct {
	SlackWebhookUrl string `envconfig:"SLACK_WEBHOOK" required:"true"`
	Channel         string `envconfig:"CHANNEL" required:"true"`
	StorageScope    string `envconfig:"STORAGE_SCOPE" required:"true"`
	Project         string `envconfig:"PROJECT" required:"true"`
}

func LoadConfig() (*Config, error) {
	var conf Config
	if err := envconfig.Process("", &conf); err != nil {
		return nil, err
	}
	return &conf, nil
}
