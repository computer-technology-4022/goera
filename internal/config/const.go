package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

func Init() {
	err := godotenv.Load()
	if err != nil {
		log.Printf("Warning: Error loading .env file: %v", err)
	}

	DBHost = getEnv("DB_HOST", DBHost)
	DBUser = getEnv("DB_USER", DBUser)
	DBPassword = getEnv("DB_PASSWORD", DBPassword)
	DBName = getEnv("DB_NAME", DBName)
	DBPort = getEnv("DB_PORT", DBPort)
	DBSSLMode = getEnv("DB_SSL_MODE", DBSSLMode)
}

const (
	ServerPort      = ":5000"
	StaticRouterDir = "web/static"
	StaticRouter    = "/static/"
)

var (
	DBHost     = "localhost"
	DBUser     = "goera_user"
	DBPassword = ""
	DBName     = "goera"
	DBPort     = "5432"
	DBSSLMode  = "disable"
)

var ProtectedPrefixes = []string{
	"/questions",
	"/profile",
	"/question",
	"/api/user",
	"/submissions",
	"/createQuestion",
}

// getEnv returns the value of an environment variable or a default value if not set
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
