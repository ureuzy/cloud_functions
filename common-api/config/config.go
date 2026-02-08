package config

import "github.com/kelseyhightower/envconfig"

type Config struct {
	Port              string `envconfig:"PORT" default:"8080"`
	BucketName        string `envconfig:"BUCKET_NAME" required:"true"`
	ProjectID         string `envconfig:"PROJECT_ID" required:"true"`
	FirebaseProjectID string `envconfig:"FIREBASE_PROJECT_ID" required:"true"`
}

func LoadConfig() (*Config, error) {
	var conf Config
	if err := envconfig.Process("", &conf); err != nil {
		return nil, err
	}
	return &conf, nil
}
