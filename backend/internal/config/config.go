package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port                   string
	DBDriver               string
	DBDSN                  string
	DBAutoMigrate          bool
	JWTSecret              string
	JWTTTL                 time.Duration
	CORSOrigins            []string
	RateLimitPerMinute     int
	PlannerMaxIterations   int
	LogLevel               string
	GracefulShutdownPeriod time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		Port:                   env("PORT", "8080"),
		DBDriver:               strings.ToLower(env("DB_DRIVER", "postgres")),
		DBDSN:                  env("DB_DSN", "host=localhost user=thermal_planner password=thermal_planner_dev dbname=thermal_planner port=5432 sslmode=disable"),
		DBAutoMigrate:          envBool("DB_AUTO_MIGRATE", true),
		JWTSecret:              env("JWT_SECRET", "development-secret-change-me-at-least-32-bytes"),
		JWTTTL:                 time.Duration(envInt("JWT_TTL_MINUTES", 480)) * time.Minute,
		CORSOrigins:            splitCSV(env("CORS_ORIGINS", "http://localhost:18523")),
		RateLimitPerMinute:     envInt("RATE_LIMIT_PER_MINUTE", 180),
		PlannerMaxIterations:   envInt("PLANNER_MAX_ITERATIONS", 500),
		LogLevel:               strings.ToLower(env("LOG_LEVEL", "info")),
		GracefulShutdownPeriod: time.Duration(envInt("SHUTDOWN_TIMEOUT_SECONDS", 10)) * time.Second,
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	if c.Port == "" {
		return errors.New("PORT must not be empty")
	}
	if c.DBDriver != "postgres" && c.DBDriver != "sqlite" {
		return fmt.Errorf("unsupported DB_DRIVER %q", c.DBDriver)
	}
	if strings.TrimSpace(c.DBDSN) == "" {
		return errors.New("DB_DSN must not be empty")
	}
	if len(c.JWTSecret) < 32 {
		return errors.New("JWT_SECRET must contain at least 32 characters")
	}
	if c.RateLimitPerMinute < 1 || c.RateLimitPerMinute > 10000 {
		return errors.New("RATE_LIMIT_PER_MINUTE must be between 1 and 10000")
	}
	if c.PlannerMaxIterations < 10 || c.PlannerMaxIterations > 10000 {
		return errors.New("PLANNER_MAX_ITERATIONS must be between 10 and 10000")
	}
	return nil
}

func env(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return fallback
}

func envInt(key string, fallback int) int {
	raw := env(key, "")
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

func envBool(key string, fallback bool) bool {
	raw := env(key, "")
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback
	}
	return value
}

func splitCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		if value := strings.TrimSpace(part); value != "" {
			values = append(values, value)
		}
	}
	return values
}
