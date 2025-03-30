package main

import (
	"fmt"
	"net/http"

	"github.com/computer-technology-4022/goera/internal/config"
	handler "github.com/computer-technology-4022/goera/internal/handlers"
)

func main() {
	http.Handle(config.StaticRouter, http.
		StripPrefix(config.StaticRouter, http.FileServer(http.Dir(config.StaticRouterDir))))
	http.HandleFunc("/", handler.WelcomeHandler)
	http.HandleFunc("/login", handler.LoginHandler)
	http.HandleFunc("/signUp", handler.SignUpHandler)
	http.HandleFunc("/questions", handler.QuestionsHandler)
	http.HandleFunc("/question", handler.QuestionHandler)
	http.HandleFunc("/submissions", handler.SubmissionPageHandler)
	http.HandleFunc("/createQuestion", handler.QuestionCreatorHandler)
	fmt.Println("Server is running on http://localhost:8080")
	http.ListenAndServe(config.ServerPort, nil)
}
