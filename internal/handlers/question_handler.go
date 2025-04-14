package handler

import (
	"fmt"
	"html/template"
	"log"
	"net/http"

	"github.com/computer-technology-4022/goera/internal/auth"
	"github.com/computer-technology-4022/goera/internal/models"
	"github.com/computer-technology-4022/goera/internal/utils"
	"github.com/gorilla/mux"
	// "strconv"
)

type QuestionPageData struct {
	Title          string
	TimeLimit      int
	MemoryLimit    int
	Statement      string
	IsAdmin        bool
	IsPublished    bool
	IsOwner        bool
	QuestionID     uint
	ErrorMessage   string
	SuccessMessage string
	ExampleInput   string
	ExampleOutput  string
	CurrentUserID  uint
}

func QuestionHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	apiPath := fmt.Sprintf("/api/questions/%s", id)
	apiClient := utils.GetAPIClient()
	var question models.Question
	err := apiClient.Get(r, apiPath,&question)
	if err != nil {
		log.Printf("Error fetching questions: %v", err)
		http.Error(w, "Failed to fetch questions", http.StatusInternalServerError)
		return
	}

	// Check for error parameters
	errorParam := r.URL.Query().Get("error")
	var errorMessage string = ""

	switch errorParam {
	case "already_published":
		errorMessage = "This question is already published."
	case "already_unpublished":
		errorMessage = "This question is already unpublished."
	}

	// Check for success parameters
	successParam := r.URL.Query().Get("success")
	var successMessage string = ""

	switch successParam {
	case "published":
		successMessage = "The question was successfully published."
	case "unpublished":
		successMessage = "The question was successfully unpublished."
	}

	data := QuestionPageData{
		Title:          question.Title,
		TimeLimit:      question.TimeLimit,
		MemoryLimit:    question.MemoryLimit,
		Statement:      question.Content,
		IsAdmin:        false,
		IsOwner:        false,
		IsPublished:    question.Published,
		QuestionID:     question.ID,
		ErrorMessage:   errorMessage,
		SuccessMessage: successMessage,
		ExampleInput:   question.ExampleInput,
		ExampleOutput:  question.ExampleOutput,
	}
	userID, exists := auth.UserIDFromContext(r.Context())
	if exists {
		data.CurrentUserID = userID
		user, err := auth.GetUserFromContext(r.Context())
		if err == nil {
			data.IsAdmin = user.Role == models.AdminRole
		}
		data.IsOwner = question.UserID == userID
	}

	funcMap := template.FuncMap{}

	tmpl := template.Must(template.New("question.html").
		Funcs(funcMap).ParseFiles("web/templates/question.html", "web/templates/base.html"))

	err = tmpl.Execute(w, data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
