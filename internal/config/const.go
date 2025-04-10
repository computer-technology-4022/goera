package config

const (
	ServerPort      = ":5000"
	StaticRouterDir = "web/static"
	StaticRouter    = "/static/"

	// Database configuration
	DBHost     = "localhost"
	DBUser     = "goera_user"
	DBPassword = "GoerA2225*&"               // Empty password if you haven't set one
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
