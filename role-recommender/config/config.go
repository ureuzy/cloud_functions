package config

import "github.com/kelseyhightower/envconfig"

type Config struct {
	SheetID string `envconfig:"SHEET_ID" required:"true"`
}

func LoadConfig() (*Config, error) {
	var conf Config
	if err := envconfig.Process("", &conf); err != nil {
		return nil, err
	}
	return &conf, nil
}
