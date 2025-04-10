package database

import (
	"fmt"

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
		return err
	}

	models.MigrateQuestion(DB)
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
