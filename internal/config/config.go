package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
)

type Config struct {
	Env      string
	Host     string
	Port     int
	LogLevel slog.Level

	DBHost    string
	DBPort    int
	DBUser    string
	DBPass    string
	DBName    string
	DBSSLMode string

	JWTSecret     string
	JWTTTLMinutes int

	CORSOrigins string
}

func MustLoad() Config {
	cfg, err := Load()
	if err != nil {
		panic(err)
	}
	return cfg
}

func Load() (Config, error) {
	cfg := Config{}
	cfg.Env = getenv("APP_ENV", "local")
	cfg.Host = getenv("APP_HOST", "0.0.0.0")
	cfg.Port = getenvInt("APP_PORT", 8080)
	cfg.LogLevel = parseLogLevel(getenv("APP_LOG_LEVEL", "info"))

	cfg.DBHost = getenv("DB_HOST", "postgres")
	cfg.DBPort = getenvInt("DB_PORT", 5432)
	cfg.DBUser = getenv("DB_USER", "ws")
	cfg.DBPass = getenv("DB_PASSWORD", "ws")
	cfg.DBName = getenv("DB_NAME", "ws_messenger")
	cfg.DBSSLMode = getenv("DB_SSLMODE", "disable")

	cfg.JWTSecret = getenv("JWT_SECRET", "change_me")
	cfg.JWTTTLMinutes = getenvInt("JWT_TTL_MINUTES", 60)

	cfg.CORSOrigins = getenv("CORS_ORIGINS", "*")

	return cfg, nil
}

func (c Config) Addr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

func (c Config) DBURL() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		c.DBUser,
		c.DBPass,
		c.DBHost,
		c.DBPort,
		c.DBName,
		c.DBSSLMode,
	)
}

func getenv(key, def string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return def
}

func getenvInt(key string, def int) int {
	if v, ok := os.LookupEnv(key); ok {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return def
}

func parseLogLevel(s string) slog.Level {
	switch s {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
