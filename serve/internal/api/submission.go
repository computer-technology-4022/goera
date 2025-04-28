package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"goera/serve/internal/auth"
	"goera/serve/internal/database"
	"goera/serve/internal/models"

	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

// SubmissionRequest represents the request body for creating a submission
type SubmissionRequest struct {
	Code       string `json:"code"`
	Language   string `json:"language"`
	QuestionID uint   `json:"questionId"`
}

type PendingSubmission struct {
	SubmissionID  uint              `json:"submissionId"`
	SourceCode  string            `json:"sourceCode"`
	TestCases   []models.TestCase `json:"testCases"`
	TimeLimit   string            `json:"timeLimit"`
	MemoryLimit string            `json:"memoryLimit"`
	CPUCount    string            `json:"cpuCount"`
	DockerImage string            `json:"dockerImage"`
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

	var question models.Question
	result := db.Preload("TestCases").First(&question, submissionReq.QuestionID)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			http.Error(w, "Question not found", http.StatusNotFound)
		} else {
			log.Printf("Database error: %v", result.Error)
			http.Error(w, "Failed to retrieve question", http.StatusInternalServerError)
		}
		return
	}

	// Validate test cases
	if len(question.TestCases) == 0 {
		log.Printf("No test cases found for question ID %d", submissionReq.QuestionID)
		http.Error(w, "Question has no test cases", http.StatusBadRequest)
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

	// Prepare submission for judge service
	pendingSubmission := PendingSubmission{
		SubmissionID:  submission.ID,
		SourceCode:  submission.Code,
		TestCases:   question.TestCases,
		TimeLimit:   fmt.Sprintf("%dms", question.TimeLimit),
		MemoryLimit: fmt.Sprintf("%d", question.MemoryLimit),
		CPUCount:    "1.0",
		DockerImage: "go-judge-runner:latest",
	}

	payload, err := json.Marshal(pendingSubmission)
	if err != nil {
		log.Printf("Failed to marshal judge submission: %v", err)
		http.Error(w, "Failed to prepare submission for judging", http.StatusInternalServerError)
		return
	}

	req, err := http.NewRequest("POST", "http://localhost:8080/submit", bytes.NewReader(payload))
	if err != nil {
		log.Printf("Failed to create judge request: %v", err)
		http.Error(w, "Failed to send submission to judge", http.StatusInternalServerError)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Failed to send submission to judge: %v", err)
		http.Error(w, "Judge service unavailable", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("Judge service error: %d %s", resp.StatusCode, string(body))
		http.Error(w, fmt.Sprintf("Judge service rejected submission: %s", string(body)), http.StatusInternalServerError)
		return
	}

	// Update submission status to Judging
	submission.JudgeStatus = models.Judging
	result = db.Save(&submission)
	if result.Error != nil {
		log.Printf("Failed to update submission status: %v", result.Error)
		// Note: We don't fail the request here since the judge has accepted it
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(submission); err != nil {
		log.Printf("JSON encoding error: %v", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}
