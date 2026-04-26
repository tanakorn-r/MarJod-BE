// @title           Finance Chat API
// @version         1.0
// @description     Chat-based income/expense tracker powered by Ollama (local LLM).
// @host            localhost:8080
// @BasePath        /

package main

import (
	"finance-chat/config"
	"finance-chat/database"
	"finance-chat/router"
)

func main() {
	cfg := config.Get()
	db := database.Get()
	database.Migrate(db)

	r := router.Setup(db, cfg)
	r.Run(":" + cfg.Port)
}
