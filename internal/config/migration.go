package config

import (
	"log"

	"github.com/ganiramadhan/ganipedia/backend/internal/model"
)

func RunMigrations() {
	if DB == nil {
		log.Println("Warning: Database not connected, skipping migrations")
		return
	}

	if err := DB.AutoMigrate(&model.Product{}, &model.User{}); err != nil {
		log.Printf("AutoMigrate warning: %v", err)
	}
	indexes := []string{
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email ON users(email) WHERE deleted_at IS NULL`,
		`CREATE INDEX IF NOT EXISTS idx_users_deleted_at ON users(deleted_at)`,
		`CREATE INDEX IF NOT EXISTS idx_products_deleted_at ON products(deleted_at)`,
	}
	for _, idx := range indexes {
		if err := DB.Exec(idx).Error; err != nil {
			log.Printf("Warning: index creation: %v", err)
		}
	}

	log.Println("Database migration completed successfully")
}
