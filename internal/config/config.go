package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	APIPort         string
	DatabaseURL     string
	KafkaBrokers    []string
	WebhookURL      string
	RateLimitPerSec int
	MaxRetries      int
	BaseRetryDelay  time.Duration
	JaegerEndpoint  string
	ServiceName     string
}

func Load() *Config {
	return &Config{
		APIPort:         envOrDefault("API_PORT", ":8080"),
		DatabaseURL:     envOrDefault("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/notifications?sslmode=disable"),
		KafkaBrokers:    strings.Split(envOrDefault("KAFKA_BROKERS", "localhost:9092"), ","),
		WebhookURL:      envOrDefault("WEBHOOK_URL", "https://webhook.site/test"),
		RateLimitPerSec: envOrDefaultInt("RATE_LIMIT_PER_SEC", 100),
		MaxRetries:      envOrDefaultInt("MAX_RETRIES", 3),
		BaseRetryDelay:  envOrDefaultDuration("BASE_RETRY_DELAY", 5*time.Second),
		JaegerEndpoint:  envOrDefault("JAEGER_ENDPOINT", "http://localhost:4318/v1/traces"),
		ServiceName:     envOrDefault("SERVICE_NAME", "insider-one-notification"),
	}
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envOrDefaultInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

func envOrDefaultDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}
