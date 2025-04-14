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
)

type QuestionEditData struct {
	Question      models.Question
	ErrorMessage  string
	CurrentUserID uint
}

func QuestionEditHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	questionID := vars["id"]

	// Get the current user ID from context
	userID, exists := auth.UserIDFromContext(r.Context())
	if !exists {
		http.Redirect(w, r, "/login?error=unauthorized", http.StatusSeeOther)
		return
	}

	// Get user details to check if admin
	user, err := auth.GetUserFromContext(r.Context())
	if err != nil {
		log.Printf("Error getting user from context: %v", err)
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}

	// Fetch the question from the API
	apiPath := fmt.Sprintf("/api/questions/%s", questionID)
	apiClient := utils.GetAPIClient()
	var question models.Question
	err = apiClient.Get(r, apiPath,&question)
	if err != nil {
		log.Printf("Error fetching question: %v", err)
		http.Error(w, "Failed to fetch question", http.StatusInternalServerError)
		return
	}

	// Check if user is authorized to edit the question
	// User must be either an admin or the owner of the question
	if user.Role != models.AdminRole && question.UserID != userID {
		http.Error(w, "Unauthorized to edit this question", http.StatusForbidden)
		return
	}

	// Prepare data for the template
	data := QuestionEditData{
		Question:      question,
		CurrentUserID: userID,
	}

	// Parse and execute the template
	tmpl, err := template.ParseFiles("web/templates/questionEditForm.html")
	if err != nil {
		log.Printf("Error parsing template: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = tmpl.Execute(w, data)
	if err != nil {
		log.Printf("Error executing template: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
