package router

import (
	"encoding/json"
	"finance-chat/agent"
	"finance-chat/config"
	"finance-chat/controller"
	"finance-chat/middleware"
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
	// Repositories
	txRepo := repository.NewTransactionRepository(db)
	correctionRepo := repository.NewCorrectionRepository(db)
	profileRepo := repository.NewBehaviorProfileRepository(db)
	planRepo := repository.NewUserPlanRepository(db)
	questRepo := repository.NewQuestRepository(db)
	walletRepo := repository.NewWalletRepository(db)

	// Services
	llm := service.NewOpenAIClient(cfg)
	analyticsLLM := service.NewOpenAIInternalClient(cfg)
	lineSvc := service.NewLineService(cfg)

	// Agent dependencies
	agentDeps := agent.AgentDeps{
		LLM:          llm,
		AnalyticsLLM: analyticsLLM,
		LineService:  lineSvc,
		TxRepo:       txRepo,
		ProfileRepo:  profileRepo,
		PlanRepo:     planRepo,
		WalletRepo:   walletRepo,
	}

	// Transaction service with agent pipeline
	txSvc := service.NewTransactionService(txRepo, correctionRepo, profileRepo, planRepo, walletRepo, agentDeps)
	questSvc := service.NewQuestService(questRepo, txRepo)
	txSvc.SetQuestProgressEvaluator(questSvc)
	walletSvc := service.NewWalletService(walletRepo)
	// Controllers
	txCtrl := controller.NewTransactionController(txSvc)
	webhookCtrl := controller.NewWebhookController(txSvc, lineSvc, txRepo)
	questCtrl := controller.NewQuestController(questSvc)
	walletCtrl := controller.NewWalletController(walletSvc, txSvc)

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
	api.Use(middleware.RequireLineAuth(lineSvc, cfg))
	{
		api.POST("/chat", txCtrl.Chat)
		api.POST("/chat/stream", txCtrl.ChatStream)
		api.GET("/transactions", txCtrl.List)
		api.GET("/transactions/by-category", txCtrl.ListByCategory)
		api.GET("/transactions/:id", txCtrl.GetByID)
		api.DELETE("/transactions/:id", txCtrl.Delete)
		api.PATCH("/transactions/:id/correct", txCtrl.Correct)
		api.GET("/summary", txCtrl.Summary)
		api.GET("/analytics", txCtrl.Analytics)
		api.GET("/analytics/trend", txCtrl.AnalyticsTrend)
		api.GET("/analytics/dna", txCtrl.SpendingDNA)
		api.POST("/analytics/insight", txCtrl.AnalyticsInsight)
		api.GET("/corrections", txCtrl.ListCorrections)
		api.DELETE("/corrections/:id", txCtrl.DeleteCorrection)
		api.GET("/quests", questCtrl.GetQuests)
		api.POST("/quests/generate", questCtrl.GenerateQuests)
		api.POST("/quests/reroll", questCtrl.RerollQuests)
		api.GET("/quest-presets", questCtrl.ListQuestPresets)
		api.POST("/quest-presets", questCtrl.CreateQuestPreset)
		api.PATCH("/quest-presets/:id", questCtrl.UpdateQuestPreset)
		api.DELETE("/quest-presets/:id", questCtrl.DeleteQuestPreset)
		api.GET("/wallets", walletCtrl.ListWallets)
		api.POST("/wallets", walletCtrl.CreateWallet)
		api.GET("/wallets/current", walletCtrl.GetCurrentWallet)
		api.PATCH("/wallets/current", walletCtrl.SetCurrentWallet)
		api.PATCH("/wallets/:id", walletCtrl.UpdateWallet)
	}

	return r
}
