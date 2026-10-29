package config

import (
	"log"
	"log/slog"
	"os"
	"time"
)

type Config struct {
	Server     ServerConfig
	Postgres   PostgresConfig
	Auth       AuthConfig
	Generation GenerationConfig
	Outbox     OutboxConfig
	RabbitMQ   RabbitMQConfig
	Logging    LoggingConfig
}

type ServerConfig struct {
	Host string
	Port string
}

type PostgresConfig struct {
	Host     string
	Port     string
	Name     string
	User     string
	Password string
}

type AuthConfig struct {
	JWTSecret string
}

type GenerationConfig struct {
	Timeout time.Duration
}

type OutboxConfig struct {
	Retention time.Duration
}

type RabbitMQConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Vhost    string
}

type LoggingConfig struct {
	Level slog.Level
}

func Load() Config {
	return Config{
		Server: ServerConfig{
			Host: getEnv("APP_HOST", "0.0.0.0"),
			Port: getEnv("APP_PORT", "8080"),
		},

		Postgres: PostgresConfig{
			Host:     getEnv("POSTGRES_HOST", "localhost"),
			Port:     getEnv("POSTGRES_PORT", "5432"),
			Name:     getEnv("POSTGRES_DB", "gitflow"),
			User:     getEnv("POSTGRES_USER", "postgres"),
			Password: getEnv("POSTGRES_PASSWORD", "postgres"),
		},

		Auth: AuthConfig{
			JWTSecret: os.Getenv("JWT_SECRET"),
		},

		Generation: GenerationConfig{
			Timeout: getEnvDuration("GENERATION_TIMEOUT", 5*time.Minute),
		},

		Outbox: OutboxConfig{
			Retention: getEnvDuration("OUTBOX_RETENTION", 7*24*time.Hour),
		},

		RabbitMQ: RabbitMQConfig{
			Host:     getEnv("RABBITMQ_HOST", "localhost"),
			Port:     getEnv("RABBITMQ_PORT", "5672"),
			User:     getEnv("RABBITMQ_USER", ""),
			Password: getEnv("RABBITMQ_PASSWORD", ""),
			Vhost:    getEnv("RABBITMQ_VHOST", "/"),
		},

		Logging: LoggingConfig{
			Level: getEnvLogLevel("LOG_LEVEL", slog.LevelInfo),
		},
	}
}

func getEnv(key, def string) string {
	value := os.Getenv(key)
	if value == "" {
		return def
	}

	return value
}

func getEnvDuration(key string, def time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return def
	}

	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		log.Printf("invalid duration for %s=%q, using default %s", key, value, def)
		return def
	}

	return duration
}

func getEnvLogLevel(key string, def slog.Level) slog.Level {
	v := os.Getenv(key)

	switch v {
	case "DEBUG":
		return slog.LevelDebug
	case "INFO":
		return slog.LevelInfo
	case "WARN":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	case "":
		return def
	default:
		log.Printf("invalid log level for %s=%q, using default %s", key, v, def)
		return def
	}
}
