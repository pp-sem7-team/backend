package config

import (
	"log"
	"log/slog"
	"os"
)

type Config struct {
	Server   ServerConfig
	Postgres PostgresConfig
	Logging  LoggingConfig
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
