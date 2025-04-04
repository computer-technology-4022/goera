package config

const (
	ServerPort      = ":8080"
	StaticRouterDir = "web/static"
	StaticRouter    = "/static/"
)

var ProtectedPrefixes = []string{
	"/questions",
	"/profile",
	"/question",
	"/submissions",
	"/createQuestion",
}
