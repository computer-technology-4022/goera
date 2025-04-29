package main

import (
	"flag"
	"fmt"
	"goera/serve/internal/api"
	"goera/serve/internal/auth"
	"goera/serve/internal/config"
	"goera/serve/internal/database"
	handler "goera/serve/internal/handlers"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gorilla/mux"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: serve <command> [options]")
		fmt.Println("Commands:")
		fmt.Println("  serve    Start the server")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "serve":
		serveCmd := flag.NewFlagSet("serve", flag.ExitOnError)
		listenAddr := serveCmd.String("listen", "5000", "Port to listen on (e.g., 5000 or :5000)")
		serveCmd.Parse(os.Args[2:])

		addr := *listenAddr
		if !strings.Contains(addr, ":") {
			addr = ":" + addr
		}

		runServer(addr)

	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

func runServer(port string) {
	config.Init()
	
	// Update the configured port after config initialization
	config.ServerPort = port
	
	err := database.InitDB()
	if err != nil {
		log.Fatal(err)
		return
	}
	defer database.CloseDB()

	r := mux.NewRouter()
	r.Use(auth.Middleware)
	fs := http.FileServer(http.Dir(config.StaticRouterDir))
	r.PathPrefix(config.StaticRouter).Handler(http.StripPrefix(config.StaticRouter, fs))
	r.HandleFunc("/internalapi/judge/{id:[0-9]+}", api.ServerJudgeHandler)
	r.HandleFunc("/", handler.WelcomeHandler)
	r.HandleFunc("/login", handler.LoginHandler)
	r.HandleFunc("/signUp", handler.SignUpHandler)
	r.HandleFunc("/questions", handler.QuestionsHandler)
	r.HandleFunc("/question/{id:[0-9]+}", handler.QuestionHandler)
	r.HandleFunc("/edit/{id:[0-9]+}", handler.QuestionEditHandler)
	r.HandleFunc("/submissions", handler.SubmissionPageHandler)
	r.HandleFunc("/createQuestion", handler.QuestionCreateHandler)
	r.HandleFunc("/profile/{id:[0-9]+}", handler.ProfileHandler)

	s := r.PathPrefix("/api").Subrouter()
	s.HandleFunc("/login", api.LoginHandler).Methods("GET", "POST")
	s.HandleFunc("/register", api.RegisterHandler).Methods("GET", "POST")
	s.HandleFunc("/logout", api.LogoutHandler).Methods("GET", "POST")
	s.HandleFunc("/user/{id:[0-9]+}/promote", api.PromoteUserHandler).Methods("PUT", "POST")
	s.HandleFunc("/user/{id:[0-9]+}", api.UsersHandler).Methods("GET")

	s.HandleFunc("/questions", api.QuestionsHandler).Methods("GET", "POST")
	s.HandleFunc("/questions/{id}", api.QuestionHandler).Methods("GET", "PUT", "DELETE", "POST")
	s.HandleFunc("/questions/{id}/publish", api.PublishQuestionHandler).Methods("PUT", "POST")
	s.HandleFunc("/questions/{id}/testcase", api.TestCaseHandler).Methods("GET")

	s.HandleFunc("/submissions", api.SubmissionsHandler).Methods("GET", "POST")
	s.HandleFunc("/submissions/{id}", api.SubmissionHandler).Methods("GET")

	http.Handle("/", r)
	fmt.Printf("Server is running on http://localhost%s\n", config.ServerPort)
	http.ListenAndServe(config.ServerPort, nil)
}
