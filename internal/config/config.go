package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	APIAddr            string
	PostgresURL        string
	RedisAddr          string
	RedisPassword      string
	RedisDB            int
	SchedulerTickSec   int
	FixScriptsDir      string
	AllowedFixCommands []string
	AlertCooldownSec   int
	Environment        string
	UIDir              string
}

func Load() Config {
	return Config{
		APIAddr:            envOrDefault("API_ADDR", ":8080"),
		PostgresURL:        envOrDefault("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/kraken?sslmode=disable"),
		RedisAddr:          envOrDefault("REDIS_ADDR", "localhost:6379"),
		RedisPassword:      os.Getenv("REDIS_PASSWORD"),
		RedisDB:            envInt("REDIS_DB", 0),
		SchedulerTickSec:   envInt("SCHEDULER_TICK_SEC", 2),
		FixScriptsDir:      envOrDefault("FIX_SCRIPTS_DIR", "scripts/fixes"),
		AllowedFixCommands: envCSV("ALLOWED_FIX_COMMANDS", []string{"bash"}),
		AlertCooldownSec:   envInt("ALERT_COOLDOWN_SEC", 300),
		Environment:        envOrDefault("APP_ENV", "dev"),
		UIDir:              os.Getenv("UI_DIR"),
	}
}

func envOrDefault(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func envInt(key string, fallback int) int {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(val)
	if err != nil {
		return fallback
	}
	return parsed
}

func envCSV(key string, fallback []string) []string {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return fallback
	}
	parts := strings.Split(val, ",")
	res := make([]string, 0, len(parts))
	for _, part := range parts {
		p := strings.TrimSpace(part)
		if p != "" {
			res = append(res, p)
		}
	}
	if len(res) == 0 {
		return fallback
	}
	return res
}
