package database

import (
	"fmt"
	"log"

	"github.com/computer-technology-4022/goera/internal/config"
	"github.com/computer-technology-4022/goera/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func InitDB() error {
	var err error
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
		config.DBHost, config.DBUser, config.DBPassword, config.DBName, config.DBPort, config.DBSSLMode)
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Printf("Error: Failed to connect as application user '%s': %v", config.DBUser, err)
		return fmt.Errorf("failed to connect database as user %s: %w", config.DBUser, err)
	}

	// Run migrations
	migrations := map[string]func(*gorm.DB) error{
		"Question":   models.MigrateQuestion,
		"User":       models.MigrateUser,
		"Submission": models.MigrateSubmission,
		"TestCase": models.MigrateTestCase,
	}
	for name, migrateFunc := range migrations {
		if err := migrateFunc(DB); err != nil {
			log.Printf("Error: Failed to run migration for %s: %v", name, err)
			return fmt.Errorf("failed migration for %s: %w", name, err)
		}
	}

	return nil
}

func CloseDB() error {
	db, err := DB.DB()
	if err != nil {
		return err
	}
	return db.Close()
}

func GetDB() *gorm.DB {
	return DB
}
