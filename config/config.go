package config

import (
	"os"
	"strings"
	"sync"
)

type Config struct {
	Port            string
	DBPath          string
	OllamaURL       string
	OllamaModel     string
	CORSOrigins     []string
	LineChannelToken string
	LineChannelSecret string
}

var (
	instance *Config
	once     sync.Once
)

func Get() *Config {
	once.Do(func() {
		instance = &Config{
			Port:              getEnv("PORT", "8080"),
			DBPath:            getEnv("DB_PATH", "./finance.db"),
			OllamaURL:         getEnv("OLLAMA_URL", "http://localhost:11434"),
			OllamaModel:       getEnv("OLLAMA_MODEL", "llama3.2:3b"),
			CORSOrigins:       strings.Split(getEnv("CORS_ORIGINS", "http://localhost:5173,https://showpiece-frigidly-unless.ngrok-free.dev"), ","),
			LineChannelToken:  getEnv("LINE_CHANNEL_TOKEN", "vVIBouZujK2ijnpkg1xBQlzSn6HQP/Hk3L9KPBdhYDzSjaSVbHU2uh745kPZ4RB/Tq1f8KBU4/oYlAIKCmSnD0GIecRQWPWFy3K6vEYC5PdZSgQieea+wVrJ0D6K7WA9CvaWXVfXY7LpugsNiFAT0QdB04t89/1O/w1cDnyilFU="),
			LineChannelSecret: getEnv("LINE_CHANNEL_SECRET", ""),
		}
	})
	return instance
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
