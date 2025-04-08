package main

import (
	"fmt"
	"net/http"

	"github.com/computer-technology-4022/goera/internal/api"
	"github.com/computer-technology-4022/goera/internal/auth"
	"github.com/computer-technology-4022/goera/internal/config"
	"github.com/computer-technology-4022/goera/internal/database"
	handler "github.com/computer-technology-4022/goera/internal/handlers"
	"github.com/gorilla/mux"
)

func main() {

	database.InitDB()
	r := mux.NewRouter()
	r.Use(auth.Middleware)
	fs := http.FileServer(http.Dir(config.StaticRouterDir))
	r.PathPrefix(config.StaticRouter).Handler(http.StripPrefix(config.StaticRouter, fs))
	r.HandleFunc("/", handler.WelcomeHandler)
	r.HandleFunc("/login", handler.LoginHandler)
	r.HandleFunc("/signUp", handler.SignUpHandler)m
	r.HandleFunc("/questions", handler.QuestionsHandler)
	r.HandleFunc("/question", handler.QuestionHandler)
	fmt.Println("Server is running on http://localhost:8080")

	s := r.PathPrefix("/api").Subrouter()
	s.HandleFunc("/login", api.LoginHandler).Methods("GET", "POST")
	s.HandleFunc("/register", api.RegisterHandler).Methods("GET", "POST")
	s.HandleFunc("/user", api.UsersHandler).Methods("GET", "POST")

	http.Handle("/", r)
	http.ListenAndServe(config.ServerPort, nil)
}
