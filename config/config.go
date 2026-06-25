package config

import (
	"log"
	"os"
	"strings"
	"sync"

	"github.com/joho/godotenv"
)

type Config struct {
	Port                 string
	DBPath               string // SQLite local path (used when TursoURL is empty)
	TursoURL             string // e.g. libsql://your-db.turso.io
	TursoAuthToken       string
	OpenAIAPIKey         string
	OpenAIAPIKeyInternal string
	OpenAIModel          string
	CORSOrigins          []string
	LineChannelToken     string
	LineChannelSecret    string
}

var (
	instance *Config
	once     sync.Once
)

func Get() *Config {
	once.Do(func() {
		// Load env file — defaults to .env, override with ENV_FILE=.env.production
		// Overload forces file values to win over any pre-existing shell env vars.
		envFile := os.Getenv("ENV_FILE")
		if envFile == "" {
			envFile = ".env"
		}
		if err := godotenv.Overload(envFile); err != nil {
			log.Printf("[config] %s not found, using environment variables", envFile)
		}

		instance = &Config{
			Port:                 getEnv("PORT", "8080"),
			DBPath:               getEnv("DB_PATH", "./finance.db"),
			TursoURL:             getEnv("TURSO_URL", ""),
			TursoAuthToken:       getEnv("TURSO_AUTH_TOKEN", ""),
			OpenAIAPIKey:         getEnv("OPENAI_API_KEY", ""),
			OpenAIAPIKeyInternal: getEnv("OPENAI_API_KEY_INTERNAL", ""),
			OpenAIModel:          getEnv("OPENAI_MODEL", "gpt-4o-mini"),
			CORSOrigins:          strings.Split(getEnv("CORS_ORIGINS", "http://localhost:5173"), ","),
			LineChannelToken:     getEnv("LINE_CHANNEL_TOKEN", ""),
			LineChannelSecret:    getEnv("LINE_CHANNEL_SECRET", ""),
		}
	})
	return instance
}

// getEnv returns the env var value or the fallback if not set.
func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// mustEnv returns the env var value or panics with a clear message.
// Use for values that are required for the app to function.
func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("[config] required environment variable %q is not set", key)
	}
	return v
}
