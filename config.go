package main

import (
	"log"
	"os"
)

type Config struct {
	NodeStateURL     string
	RabbitmqURL      string
	RabbitmqUsername string
	RabbitmqPassword string
	UploadServerURL  string
}

func mustGetConfigFromEnv() *Config {
	return &Config{
		NodeStateURL:     mustGetEnv("NODE_STATE_API"),
		RabbitmqURL:      mustGetEnv("RMQ_URL"),
		RabbitmqUsername: mustGetEnv("RMQ_USERNAME"),
		RabbitmqPassword: mustGetEnv("RMQ_PASSWORD"),
		UploadServerURL:  mustGetEnv("UPLOADER_URL"),
	}
}

func mustGetEnv(key string) string {
	value, ok := os.LookupEnv(key)
	if !ok || value == "" {
		log.Fatalf("environment variable %s must be provided", key)
	}
	if value == "" {
		log.Fatalf("environment variable %s must be non-empty", key)
	}
	return value
}
