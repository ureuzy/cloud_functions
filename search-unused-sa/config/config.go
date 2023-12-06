package config

import "github.com/kelseyhightower/envconfig"

type Config struct {
	DaysAfterCreation int `envconfig:"DAYS_AFTER_CREATION" default:"7"`
}

func LoadConfig() (*Config, error) {
	var conf Config
	if err := envconfig.Process("", &conf); err != nil {
		return nil, err
	}
	return &conf, nil
}
