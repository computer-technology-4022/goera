package main

import (
	"fmt"
	"net/http"

	"github.com/computer-technology-4022/goera/internal/config"
	handler "github.com/computer-technology-4022/goera/internal/handlers"
	"github.com/gorilla/mux"
)

func main() {
	router := mux.NewRouter()

	// Serve static files
	router.PathPrefix(config.StaticRouter).Handler(
		http.StripPrefix(config.StaticRouter, http.FileServer(http.Dir(config.StaticRouterDir))))

	// Define routes
	router.HandleFunc("/", handler.WelcomeHandler)
	router.HandleFunc("/login", handler.LoginHandler)
	router.HandleFunc("/signUp", handler.SignUpHandler)
	router.HandleFunc("/questions", handler.QuestionsHandler)
	router.HandleFunc("/question/{id:[0-9]+}", handler.QuestionHandler)
	router.HandleFunc("/submissions", handler.SubmissionPageHandler)
	router.HandleFunc("/createQuestion", handler.QuestionCreatorHandler)
	router.HandleFunc("/profile/{id:[0-9]+}", handler.ProfileHandler)

	fmt.Println("Server is running on http://localhost:8080")
	http.ListenAndServe(config.ServerPort, router)
}
