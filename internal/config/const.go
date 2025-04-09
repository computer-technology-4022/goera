package config

const (
	ServerPort      = ":5000"
	StaticRouterDir = "web/static"
	StaticRouter    = "/static/"
)

var ProtectedPrefixes = []string{
	"/questions",
	"/profile",
	"/question",
	"/api/user",
	"/submissions",
	"/createQuestion",
}
