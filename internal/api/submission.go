package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/computer-technology-4022/goera/internal/auth"
	"github.com/computer-technology-4022/goera/internal/database"
	"github.com/computer-technology-4022/goera/internal/models"
	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

// SubmissionRequest represents the request body for creating a submission
type SubmissionRequest struct {
	Code       string `json:"code"`
	Language   string `json:"language"`
	QuestionID uint   `json:"questionId"`
}

// SubmissionsHandler handles all requests to /api/submissions
func SubmissionsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		getUserSubmissions(w, r)
	case http.MethodPost:
		createSubmission(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// SubmissionHandler handles all requests to /api/submissions/{id}
func SubmissionHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		getSubmissionByID(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// getUserSubmissions retrieves all submissions for the current user
func getUserSubmissions(w http.ResponseWriter, r *http.Request) {
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

	// Parse pagination parameters
	page := 1
	pageSize := 5 // Default page size for submissions

	if pageParam := r.URL.Query().Get("page"); pageParam != "" {
		if parsedPage, err := strconv.Atoi(pageParam); err == nil && parsedPage > 0 {
			page = parsedPage
		}
	}

	if pageSizeParam := r.URL.Query().Get("page_size"); pageSizeParam != "" {
		if parsedPageSize, err := strconv.Atoi(pageSizeParam); err == nil && parsedPageSize > 0 && parsedPageSize <= 100 {
			pageSize = parsedPageSize
		}
	}

	offset := (page - 1) * pageSize

	// Start with a query for the current user's submissions
	query := db.Where("user_id = ?", userID)

	// Handle query parameters for filtering
	questionIDStr := r.URL.Query().Get("questionId")
	if questionIDStr != "" {
		questionID, err := strconv.Atoi(questionIDStr)
		if err != nil {
			http.Error(w, "Invalid question ID", http.StatusBadRequest)
			return
		}

		// Apply filter directly in database query
		query = query.Where("question_id = ?", questionID)
	}

	// Count total matching submissions
	var totalItems int64
	if err := query.Model(&models.Submission{}).Count(&totalItems).Error; err != nil {
		log.Printf("Database error counting submissions: %v", err)
		http.Error(w, "Failed to count submissions", http.StatusInternalServerError)
		return
	}

	// Calculate total pages
	totalPages := int((totalItems + int64(pageSize) - 1) / int64(pageSize))

	// Order by submission time (newest first) and get paginated results
	var submissions []models.Submission
	result := query.Order("submission_time DESC").Limit(pageSize).Offset(offset).Find(&submissions)
	if result.Error != nil {
		log.Printf("Database error: %v", result.Error)
		http.Error(w, "Failed to retrieve submissions", http.StatusInternalServerError)
		return
	}

	// Create paginated response
	response := PaginatedResponse{
		Data:       submissions,
		Page:       page,
		PageSize:   pageSize,
		TotalItems: totalItems,
		TotalPages: totalPages,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("JSON encoding error: %v", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// getSubmissionByID retrieves a submission by ID
func getSubmissionByID(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid submission ID", http.StatusBadRequest)
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

	var submission models.Submission
	result := db.First(&submission, id)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			http.Error(w, "Submission not found", http.StatusNotFound)
		} else {
			log.Printf("Database error: %v", result.Error)
			http.Error(w, "Failed to retrieve submission", http.StatusInternalServerError)
		}
		return
	}

	// Users can only see their own submissions
	if submission.UserID != userID {
		http.Error(w, "Unauthorized to view this submission", http.StatusForbidden)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(submission); err != nil {
		log.Printf("JSON encoding error: %v", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// createSubmission creates a new submission
func createSubmission(w http.ResponseWriter, r *http.Request) {
	var submissionReq SubmissionRequest
	if err := json.NewDecoder(r.Body).Decode(&submissionReq); err != nil {
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

	// Check if the question exists
	var question models.Question
	result := db.First(&question, submissionReq.QuestionID)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			http.Error(w, "Question not found", http.StatusNotFound)
		} else {
			log.Printf("Database error: %v", result.Error)
			http.Error(w, "Failed to retrieve question", http.StatusInternalServerError)
		}
		return
	}

	// Create the submission
	submission := models.Submission{
		Code:           submissionReq.Code,
		Language:       submissionReq.Language,
		JudgeStatus:    models.Pending,
		SubmissionTime: time.Now(),
		QuestionID:     submissionReq.QuestionID,
		QuestionName:   question.Title,
		UserID:         userID,
	}

	result = db.Create(&submission)
	if result.Error != nil {
		log.Printf("Database error: %v", result.Error)
		http.Error(w, "Failed to create submission", http.StatusInternalServerError)
		return
	}

	// TODO: Queue the submission for judging if there's a judge service

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(submission); err != nil {
		log.Printf("JSON encoding error: %v", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}
