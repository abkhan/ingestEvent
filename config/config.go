package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Server struct {
		Port string
	}
	Database struct {
		Host     string
		Port     string
		User     string
		Password string
		Name     string
	}
	Stores struct {
		Postgres bool
		NDJSON   bool
	}
	Middlewares struct {
		APIKey         string
		RateLimit      int // requests per second
		CircuitBreaker struct {
			FailureThreshold int
			Timeout          time.Duration
		}
	}
}

func Load() *Config {
	cfg := &Config{}

	// Server
	cfg.Server.Port = getEnv("PORT", "8080")

	// Database
	cfg.Database.Host = getEnv("DB_HOST", "localhost")
	cfg.Database.Port = getEnv("DB_PORT", "5432")
	cfg.Database.User = getEnv("DB_USER", "user")
	cfg.Database.Password = getEnv("DB_PASSWORD", "password")
	cfg.Database.Name = getEnv("DB_NAME", "fulcrum")

	// Stores
	cfg.Stores.Postgres = getEnv("STORE_POSTGRES", "true") == "true"
	cfg.Stores.NDJSON = getEnv("STORE_NDJSON", "true") == "true"

	// Middlewares
	cfg.Middlewares.APIKey = getEnv("API_KEY", "default-key")
	cfg.Middlewares.RateLimit, _ = strconv.Atoi(getEnv("RATE_LIMIT", "100"))
	cfg.Middlewares.CircuitBreaker.FailureThreshold, _ = strconv.Atoi(getEnv("CB_FAILURE_THRESHOLD", "5"))
	cfg.Middlewares.CircuitBreaker.Timeout, _ = time.ParseDuration(getEnv("CB_TIMEOUT", "10s"))

	return cfg
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
