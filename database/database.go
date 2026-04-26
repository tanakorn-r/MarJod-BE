package database

import (
	"finance-chat/config"
	"finance-chat/model"
	"log"
	"sync"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var (
	db   *gorm.DB
	once sync.Once
)

func Get() *gorm.DB {
	once.Do(func() {
		cfg := config.Get()
		var err error
		db, err = gorm.Open(sqlite.Open(cfg.DBPath), &gorm.Config{})
		if err != nil {
			log.Fatalf("failed to connect database: %v", err)
		}
	})
	return db
}

func Migrate(db *gorm.DB) {
	if err := db.AutoMigrate(&model.Transaction{}, &model.UserCorrection{}); err != nil {
		log.Fatalf("migration failed: %v", err)
	}
}
