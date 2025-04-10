package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/computer-technology-4022/goera/internal/auth"
	"github.com/computer-technology-4022/goera/internal/database"
	"github.com/computer-technology-4022/goera/internal/models"
	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

type QuestionRequest struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

type QuestionPublishRequest struct {
	Published bool `json:"published"`
}

func QuestionsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		getQuestions(w, r)
	case http.MethodPost:
		createQuestion(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// QuestionHandler handles all requests to /api/questions/{id}
func QuestionHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		getQuestionByID(w, r)
	case http.MethodPut:
		updateQuestion(w, r)
	case http.MethodDelete:
		deleteQuestion(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func PublishQuestionHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPut:
		publishQuestion(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func getQuestions(w http.ResponseWriter, r *http.Request) {
	db := database.GetDB()
	if db == nil {
		log.Println("Database connection is nil")
		http.Error(w, "Database connection error", http.StatusInternalServerError)
		return
	}

	userID, userExists := auth.UserIDFromContext(r.Context())
	if !userExists {
		log.Println("User ID not found in context")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var questions []models.Question
	var user models.User
	result := db.First(&user, userID)
	if result.Error != nil {
		log.Printf("Database error: %v", result.Error)
		http.Error(w, "Failed to retrieve user", http.StatusInternalServerError)
		return
	}

	// If user is admin, return all questions
	query := db
	if user.Role != models.AdminRole {
		query = query.Where("published = ? OR user_id = ?", true, userID)
	}

	result = query.Find(&questions)
	if result.Error != nil {
		log.Printf("Database error: %v", result.Error)
		http.Error(w, "Failed to retrieve questions", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(questions); err != nil {
		log.Printf("JSON encoding error: %v", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func getQuestionByID(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid question ID", http.StatusBadRequest)
		return
	}

	db := database.GetDB()
	if db == nil {
		log.Println("Database connection is nil")
		http.Error(w, "Database connection error", http.StatusInternalServerError)
		return
	}

	userID, userExists := auth.UserIDFromContext(r.Context())
	if !userExists {
		log.Println("User ID not found in context")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var question models.Question
	result := db.First(&question, id)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			http.Error(w, "Question not found", http.StatusNotFound)
		} else {
			log.Printf("Database error: %v", result.Error)
			http.Error(w, "Failed to retrieve question", http.StatusInternalServerError)
		}
		return
	}

	var user models.User
	result = db.First(&user, userID)
	if result.Error != nil {
		log.Printf("Database error: %v", result.Error)
		http.Error(w, "Failed to retrieve user", http.StatusInternalServerError)
		return
	}

	// Users can view questions if:
	// 1. They are admin
	// 2. The question is published
	// 3. They are the owner of the question
	if !question.Published && user.Role != models.AdminRole && question.UserID != userID {
		http.Error(w, "Unauthorized to view this question", http.StatusForbidden)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(question); err != nil {
		log.Printf("JSON encoding error: %v", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func createQuestion(w http.ResponseWriter, r *http.Request) {
	var questionReq QuestionRequest
	if err := json.NewDecoder(r.Body).Decode(&questionReq); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	userID, userExists := auth.UserIDFromContext(r.Context())
	if !userExists {
		log.Println("User ID not found in context")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	question := models.Question{
		Title:     questionReq.Title,
		Content:   questionReq.Content,
		UserID:    userID,
		Published: false,
	}

	db := database.GetDB()
	if db == nil {
		log.Println("Database connection is nil")
		http.Error(w, "Database connection error", http.StatusInternalServerError)
		return
	}

	result := db.Create(&question)
	if result.Error != nil {
		log.Printf("Database error: %v", result.Error)
		http.Error(w, "Failed to create question", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(question); err != nil {
		log.Printf("JSON encoding error: %v", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func updateQuestion(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid question ID", http.StatusBadRequest)
		return
	}

	var questionReq QuestionRequest
	if err := json.NewDecoder(r.Body).Decode(&questionReq); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	userID, userExists := auth.UserIDFromContext(r.Context())
	if !userExists {
		log.Println("User ID not found in context")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	db := database.GetDB()
	if db == nil {
		log.Println("Database connection is nil")
		http.Error(w, "Database connection error", http.StatusInternalServerError)
		return
	}

	var question models.Question
	result := db.First(&question, id)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			http.Error(w, "Question not found", http.StatusNotFound)
		} else {
			log.Printf("Database error: %v", result.Error)
			http.Error(w, "Failed to retrieve question", http.StatusInternalServerError)
		}
		return
	}

	// Check if user can edit this question (either owner or admin)
	var user models.User
	result = db.First(&user, userID)
	if result.Error != nil {
		log.Printf("Database error: %v", result.Error)
		http.Error(w, "Failed to retrieve user", http.StatusInternalServerError)
		return
	}

	if question.UserID != userID && user.Role != models.AdminRole {
		http.Error(w, "Unauthorized to edit this question", http.StatusForbidden)
		return
	}

	// Update question fields
	question.Title = questionReq.Title
	question.Content = questionReq.Content

	result = db.Save(&question)
	if result.Error != nil {
		log.Printf("Database error: %v", result.Error)
		http.Error(w, "Failed to update question", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(question); err != nil {
		log.Printf("JSON encoding error: %v", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func deleteQuestion(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid question ID", http.StatusBadRequest)
		return
	}

	userID, userExists := auth.UserIDFromContext(r.Context())
	if !userExists {
		log.Println("User ID not found in context")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	db := database.GetDB()
	if db == nil {
		log.Println("Database connection is nil")
		http.Error(w, "Database connection error", http.StatusInternalServerError)
		return
	}

	var question models.Question
	result := db.First(&question, id)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			http.Error(w, "Question not found", http.StatusNotFound)
		} else {
			log.Printf("Database error: %v", result.Error)
			http.Error(w, "Failed to retrieve question", http.StatusInternalServerError)
		}
		return
	}

	var user models.User
	result = db.First(&user, userID)
	if result.Error != nil {
		log.Printf("Database error: %v", result.Error)
		http.Error(w, "Failed to retrieve user", http.StatusInternalServerError)
		return
	}

	if question.UserID != userID && user.Role != models.AdminRole {
		http.Error(w, "Unauthorized to delete this question", http.StatusForbidden)
		return
	}

	result = db.Delete(&question)
	if result.Error != nil {
		log.Printf("Database error: %v", result.Error)
		http.Error(w, "Failed to delete question", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// publishQuestion handles publishing or unpublishing a question (admin only)
func publishQuestion(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid question ID", http.StatusBadRequest)
		return
	}

	var publishReq QuestionPublishRequest
	if err := json.NewDecoder(r.Body).Decode(&publishReq); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	userID, userExists := auth.UserIDFromContext(r.Context())
	if !userExists {
		log.Println("User ID not found in context")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	db := database.GetDB()
	if db == nil {
		log.Println("Database connection is nil")
		http.Error(w, "Database connection error", http.StatusInternalServerError)
		return
	}

	// Check if user is admin
	var user models.User
	result := db.First(&user, userID)
	if result.Error != nil {
		log.Printf("Database error: %v", result.Error)
		http.Error(w, "Failed to retrieve user", http.StatusInternalServerError)
		return
	}

	if user.Role != models.AdminRole {
		http.Error(w, "Only administrators can publish or unpublish questions", http.StatusForbidden)
		return
	}

	var question models.Question
	result = db.First(&question, id)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			http.Error(w, "Question not found", http.StatusNotFound)
		} else {
			log.Printf("Database error: %v", result.Error)
			http.Error(w, "Failed to retrieve question", http.StatusInternalServerError)
		}
		return
	}

	// Update publishing status
	question.Published = publishReq.Published
	if publishReq.Published {
		publishedByID := userID
		question.PublishedBy = &publishedByID
	} else {
		question.PublishedBy = nil
	}

	result = db.Save(&question)
	if result.Error != nil {
		log.Printf("Database error: %v", result.Error)
		http.Error(w, "Failed to update question", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(question); err != nil {
		log.Printf("JSON encoding error: %v", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}
