package config

import "github.com/kelseyhightower/envconfig"

type Config struct {
	SlackBotToken     string `envconfig:"SLACK_BOT_TOKEN" required:"true"`
	Channel           string `envconfig:"CHANNEL" required:"true"`
	ProjectID         string `envconfig:"PROJECT_ID" required:"true"`           // Vertex AI用プロジェクトID
	BigQueryProjectID string `envconfig:"BIGQUERY_PROJECT_ID" required:"true"` // BigQuery用プロジェクトID
	Location          string `envconfig:"LOCATION" default:"global"`
	ModelName         string `envconfig:"GEMINI_MODEL" default:"gemini-3-flash-preview"`
	DaysAgo           int    `envconfig:"DAYS_AGO" default:"1"` // 何日前のリリースノートを取得するか
}

func LoadConfig() (*Config, error) {
	var conf Config
	if err := envconfig.Process("", &conf); err != nil {
		return nil, err
	}
	return &conf, nil
}
