package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/computer-technology-4022/goera/internal/api"
	"github.com/computer-technology-4022/goera/internal/auth"
	"github.com/computer-technology-4022/goera/internal/config"
	"github.com/computer-technology-4022/goera/internal/database"
	handler "github.com/computer-technology-4022/goera/internal/handlers"
	"github.com/gorilla/mux"
)

func main() {
	config.Init()
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

	s.HandleFunc("/submissions", api.SubmissionsHandler).Methods("GET", "POST")
	s.HandleFunc("/submissions/{id}", api.SubmissionHandler).Methods("GET")

	http.Handle("/", r)
	fmt.Println("Server is running on http://localhost:5000")
	http.ListenAndServe(config.ServerPort, nil)
}
