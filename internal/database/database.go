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

	log.Println("Attempting initial connection as 'postgres' user...")
	// First connect as postgres user to set permissions
	adminDsn := fmt.Sprintf("host=%s user=postgres password=%s dbname=%s port=%s sslmode=%s",
		config.DBHost, config.DBPassword, config.DBName, config.DBPort, config.DBSSLMode)

	adminDB, err := gorm.Open(postgres.Open(adminDsn), &gorm.Config{})
	if err != nil {
		// Log the failure of the admin connection attempt
		log.Printf("Warning: Failed to connect as 'postgres' user (permissions may not be granted): %v", err)
		// We might want to return err here depending on requirements, but currently it proceeds
	} else {
		log.Println("Connection as 'postgres' successful. Granting permissions...")
		// Try to grant permissions
		if grantErr := adminDB.Exec("GRANT ALL ON SCHEMA public TO goera_user").Error; grantErr != nil {
			log.Printf("Warning: Failed to grant schema permissions: %v", grantErr)
		}
		if grantErr := adminDB.Exec("GRANT ALL ON ALL TABLES IN SCHEMA public TO goera_user").Error; grantErr != nil {
			log.Printf("Warning: Failed to grant table permissions: %v", grantErr)
		}
		if grantErr := adminDB.Exec("GRANT ALL ON ALL SEQUENCES IN SCHEMA public TO goera_user").Error; grantErr != nil {
			log.Printf("Warning: Failed to grant sequence permissions: %v", grantErr)
		}
		log.Println("Permissions granted (if user exists). Closing 'postgres' connection.")

		// Close admin connection
		sqlDB, _ := adminDB.DB()
		if sqlDB != nil {
			sqlDB.Close()
		}
	}

	log.Printf("Attempting connection as application user '%s'...", config.DBUser)
	// Connect as application user
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
		config.DBHost, config.DBUser, config.DBPassword, config.DBName, config.DBPort, config.DBSSLMode)
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Printf("Error: Failed to connect as application user '%s': %v", config.DBUser, err)
		return fmt.Errorf("failed to connect database as user %s: %w", config.DBUser, err) // Wrap error
	}

	log.Println("Application user connection successful. Running migrations...")
	migrations := map[string]func(*gorm.DB) error{
		"Question":   models.MigrateQuestion,
		"User":       models.MigrateUser,
		"Submission": models.MigrateSubmission,
		"TestCase": models.MigrateTestCase,
	}
	for name, migrateFunc := range migrations {
		if err := migrateFunc(DB); err != nil {
			log.Printf("Error: Failed to run migration for %s: %v", name, err)
			return fmt.Errorf("failed migration for %s: %w", name, err) // Return migration errors
		}
	}

	log.Println("Database initialization and migrations successful.")
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
