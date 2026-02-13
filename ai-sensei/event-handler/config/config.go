package config

import "github.com/kelseyhightower/envconfig"

type Config struct {
	SlackBotToken      string `envconfig:"SLACK_BOT_TOKEN" required:"true"`
	SlackSigningSecret string `envconfig:"SLACK_SIGNING_SECRET" required:"true"`
	ProjectID          string `envconfig:"PROJECT_ID" required:"true"` // Vertex AI & Firestore用
	Location           string `envconfig:"LOCATION" default:"global"`
	AIProvider         string `envconfig:"AI_PROVIDER" default:"gemini"`      // "gemini" or "claude"
	ModelName          string `envconfig:"MODEL_NAME" default:"gemini-3-flash-preview"` // Model name for the selected provider
	ClaudeAPIKey       string `envconfig:"CLAUDE_API_KEY"`                     // Claude API key (required if AI_PROVIDER=claude)
	Port               string `envconfig:"PORT" default:"8080"`
	MaxRecentMessages  int    `envconfig:"MAX_RECENT_MESSAGES" default:"10"` // 詳細保持する最新メッセージ数
}

func LoadConfig() (*Config, error) {
	var conf Config
	if err := envconfig.Process("", &conf); err != nil {
		return nil, err
	}
	return &conf, nil
}
