package database

import (
	"github.com/computer-technology-4022/goera/internal/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var DB *gorm.DB

func InitDB() error {
	var err error
	DB, err = gorm.Open(sqlite.Open("goera.db"), &gorm.Config{})
	if err != nil {
		return err
	}

	err = DB.AutoMigrate(&models.User{})
	if err != nil {
		return err
	}

	DB.Model(&models.User{}).Where("role = ''").Update("role", models.RegularRole)

	err = DB.AutoMigrate(&models.Question{})
	if err != nil {
		return err
	}
	err = DB.AutoMigrate(&models.Submission{})
	if err != nil {
		return err
	}

	return nil
}

func GetDB() *gorm.DB {
	return DB
}
