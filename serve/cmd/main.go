package main

import (
	"fmt"
	"goera/serve/internal/api"
	"goera/serve/internal/auth"
	"goera/serve/internal/config"
	"goera/serve/internal/database"
	handler "goera/serve/internal/handlers"
	"log"
	"net/http"

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

	gg := mux.NewRouter()

	gg.HandleFunc("/internalapi/judge/{id:[0-9]+}", api.ServerJudgeHandler)
	gg.Use(auth.InternalAuthMiddleware)
	r := gg.NewRoute().Subrouter()
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
	s.HandleFunc("/questions/{id}/testcase", api.TestCaseHandler).Methods("GET")

	s.HandleFunc("/submissions", api.SubmissionsHandler).Methods("GET", "POST")
	s.HandleFunc("/submissions/{id}", api.SubmissionHandler).Methods("GET")

	http.Handle("/", r)
	fmt.Println("Server is running on http://localhost:5000")
	http.ListenAndServe(config.ServerPort, nil)
}
