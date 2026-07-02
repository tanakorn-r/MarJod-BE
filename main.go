// @title           Finance Chat API
// @version         1.0
// @description     Chat-based personal finance tracker powered by OpenAI.
// @host            localhost:8080
// @BasePath        /

// @securityDefinitions.apikey  BearerAuth
// @in                          header
// @name                        Authorization
// @description                LIFF ID token, e.g. "Bearer <id_token from liff.getIDToken()>"

package main

import (
	"finance-chat/config"
	"finance-chat/database"
	"finance-chat/router"
	"log"
)

func main() {
	log.Println("[startup] loading config...")
	cfg := config.Get()

	log.Println("[startup] connecting to database...")
	db := database.Get()
	database.Migrate(db)

	log.Printf("[startup] starting server on port %s", cfg.Port)
	r := router.Setup(db, cfg)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("[startup] server failed: %v", err)
	}
}
