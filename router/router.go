package router

import (
	"encoding/json"
	"finance-chat/config"
	"finance-chat/controller"
	"finance-chat/repository"
	"finance-chat/service"
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"gorm.io/gorm"

	_ "finance-chat/docs"

	swag "github.com/swaggo/swag"
)

func Setup(db *gorm.DB, cfg *config.Config) *gin.Engine {
	txRepo := repository.NewTransactionRepository(db)
	correctionRepo := repository.NewCorrectionRepository(db)
	llm := service.NewOllamaClient(cfg)
	txSvc := service.NewTransactionService(txRepo, correctionRepo, llm)
	txCtrl := controller.NewTransactionController(txSvc)
	lineSvc := service.NewLineService(cfg)
	webhookCtrl := controller.NewWebhookController(txSvc, lineSvc)

	r := gin.Default()

	r.Use(cors.New(cors.Config{
		AllowOrigins:     cfg.CORSOrigins,
		AllowMethods:     []string{"GET", "POST", "DELETE", "PATCH", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Dynamic doc.json — rewrites "host" to match the incoming request so
	// Swagger UI works correctly behind ngrok or any reverse proxy.
	r.GET("/doc.json", func(ctx *gin.Context) {
		doc, err := swag.ReadDoc()
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		var spec map[string]any
		if err := json.Unmarshal([]byte(doc), &spec); err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Use X-Forwarded-Host (set by ngrok) or fall back to the raw Host header
		host := ctx.GetHeader("X-Forwarded-Host")
		if host == "" {
			host = ctx.Request.Host
		}
		spec["host"] = host

		// Mirror the scheme so https ngrok URLs don't trigger mixed-content errors
		scheme := "http"
		if ctx.GetHeader("X-Forwarded-Proto") == "https" || ctx.Request.TLS != nil {
			scheme = "https"
		}
		spec["schemes"] = []string{scheme}

		ctx.JSON(http.StatusOK, spec)
	})

	// Point Swagger UI at our dynamic doc.json
	r.GET("/swagger/*any", ginSwagger.WrapHandler(
		swaggerFiles.Handler,
		ginSwagger.URL("/doc.json"),
	))

	// Line webhook — must be outside /api group, no auth middleware
	r.POST("/webhook", webhookCtrl.LineWebhook)

	api := r.Group("/api")
	{
		api.POST("/chat", txCtrl.Chat)
		api.POST("/chat/stream", txCtrl.ChatStream)
		api.GET("/transactions", txCtrl.List)
		api.DELETE("/transactions/:id", txCtrl.Delete)
		api.PATCH("/transactions/:id/correct", txCtrl.Correct)
		api.GET("/summary", txCtrl.Summary)
	}

	return r
}
