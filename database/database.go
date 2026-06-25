package database

import (
	"database/sql"
	"finance-chat/config"
	"finance-chat/model"
	"log"
	"os"
	"sync"

	_ "github.com/tursodatabase/libsql-client-go/libsql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var (
	db   *gorm.DB
	once sync.Once
)

func gormLogger() logger.Interface {
	return logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		logger.Config{
			LogLevel:                  logger.Error,
			IgnoreRecordNotFoundError: true,
		},
	)
}

func Get() *gorm.DB {
	once.Do(func() {
		cfg := config.Get()
		var err error

		if cfg.TursoURL != "" {
			// ── Turso (remote SQLite) ──────────────────────────────────────
			// libsql driver handles the libsql:// URL + auth token transparently
			db, err = openTurso(cfg)
		} else {
			// ── Local SQLite ───────────────────────────────────────────────
			db, err = gorm.Open(sqlite.Open(cfg.DBPath), &gorm.Config{
				Logger: gormLogger(),
			})
		}

		if err != nil {
			log.Fatalf("[database] failed to connect: %v", err)
		}
		log.Println("[database] connected")
	})
	return db
}

func openTurso(cfg *config.Config) (*gorm.DB, error) {
	connector, err := sql.Open("libsql", cfg.TursoURL+"?authToken="+cfg.TursoAuthToken)
	if err != nil {
		return nil, err
	}
	return gorm.Open(sqlite.Dialector{
		DSN:  cfg.TursoURL,
		Conn: connector,
	}, &gorm.Config{
		Logger: gormLogger(),
	})
}

func Migrate(db *gorm.DB) {
	// 🚨 UPDATED: Added &model.Wallet{} first in the sequence so it maps before dependent transactions
	if err := db.AutoMigrate(
		&model.Wallet{},
		&model.Transaction{},
		&model.UserCorrection{},
		&model.BehaviorProfile{},
		&model.UserPlan{},
		&model.UserQuestAssignment{},
		&model.UserGameProfile{},
		&model.UserWalletSelection{},
	); err != nil {
		log.Fatalf("[database] migration failed: %v", err)
	}

	// Rows created before user ownership existed must remain accessible. These
	// updates are additive and never delete or recreate application records.
	legacyModels := []any{
		&model.Transaction{},
		&model.UserCorrection{},
		&model.BehaviorProfile{},
		&model.UserPlan{},
		&model.Wallet{}, // Also added wallet here to clean up any structural testing rows safely
	}
	for _, legacyModel := range legacyModels {
		if err := db.Model(legacyModel).
			Where("user_id IS NULL OR user_id = ?", "").
			Update("user_id", model.DefaultUserID).Error; err != nil {
			log.Fatalf("[database] user_id backfill failed: %v", err)
		}
	}
	log.Println("[database] migrations applied")
}
