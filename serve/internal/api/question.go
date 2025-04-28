package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"goera/serve/internal/auth"
	"goera/serve/internal/database"
	"goera/serve/internal/models"
	"goera/serve/internal/utils"

	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

// SampleIO represents a single pair of input and output examples
type SampleIO struct {
	Input  string `json:"input"`
	Output string `json:"output"`
}

type QuestionRequest struct {
	Title         string   `json:"title"`
	Content       string   `json:"content"`
	TimeLimit     int      `json:"time_limit_ms"`
	MemoryLimit   int      `json:"memory_limit_mb"`
	SampleInputs  []string `json:"sample_inputs"`
	SampleOutputs []string `json:"sample_outputs"`
	Tags          string   `json:"tags"`
}

type QuestionPublishRequest struct {
	Published bool `json:"published"`
}

type PaginatedResponse struct {
	Data       any   `json:"data"`
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	TotalItems int64 `json:"total_items"`
	TotalPages int   `json:"total_pages"`
}

type QuestionsByIdResponse struct {
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
	// Check for method override in form submissions
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err == nil {
			if method := r.FormValue("_method"); method == "PUT" {
				r.Method = http.MethodPut
			} else if method == "DELETE" {
				r.Method = http.MethodDelete
			}
		}
	}

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
	case http.MethodPut, http.MethodPost:
		publishQuestion(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func TestCaseHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		getTestCasesByQuestionID(w, r)
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

	// Parse pagination parameters
	page := 1
	pageSize := 3

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

	var user models.User
	result := db.First(&user, userID)
	if result.Error != nil {
		log.Printf("Database error: %v", result.Error)
		http.Error(w, "Failed to retrieve user", http.StatusInternalServerError)
		return
	}

	query := db
	if user.Role != models.AdminRole {
		query = query.Where("published = ? OR user_id = ?", true, userID)
	}

	var totalItems int64
	if err := query.Model(&models.Question{}).Count(&totalItems).Error; err != nil {
		log.Printf("Database error counting questions: %v", err)
		http.Error(w, "Failed to count questions", http.StatusInternalServerError)
		return
	}

	totalPages := int((totalItems + int64(pageSize) - 1) / int64(pageSize))

	var questions []models.Question
	result = query.Limit(pageSize).Offset(offset).Find(&questions)
	if result.Error != nil {
		log.Printf("Database error: %v", result.Error)
		http.Error(w, "Failed to retrieve questions", http.StatusInternalServerError)
		return
	}

	response := PaginatedResponse{
		Data:       questions,
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

	// Process form data using our utility function
	formProcessor := func(r *http.Request) (interface{}, error) {
		var formReq QuestionRequest

		formReq.Title = r.FormValue("title")
		formReq.Content = r.FormValue("content")

		// Parse time limit
		if timeLimitStr := r.FormValue("time_limit_ms"); timeLimitStr != "" {
			timeLimit, err := strconv.Atoi(timeLimitStr)
			if err != nil {
				return nil, fmt.Errorf("invalid time limit: %v", err)
			}
			formReq.TimeLimit = timeLimit
		}

		// Parse memory limit
		if memoryLimitStr := r.FormValue("memory_limit_mb"); memoryLimitStr != "" {
			memoryLimit, err := strconv.Atoi(memoryLimitStr)
			if err != nil {
				return nil, fmt.Errorf("invalid memory limit: %v", err)
			}
			formReq.MemoryLimit = memoryLimit
		}

		// Get sample inputs and outputs
		formReq.SampleInputs = r.Form["sample_inputs[]"]
		formReq.SampleOutputs = r.Form["sample_outputs[]"]

		// Get tags
		formReq.Tags = r.FormValue("tags")

		// Validate required fields
		if formReq.Title == "" || formReq.Content == "" {
			return nil, fmt.Errorf("title and content are required")
		}

		log.Println("Form data processed successfully:", formReq.Title)
		log.Println("Sample inputs:", formReq.SampleInputs)
		log.Println("Sample outputs:", formReq.SampleOutputs)

		return formReq, nil
	}

	result, err := utils.ProcessRequestData(r, &questionReq, formProcessor)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// If the result came from form processing, we need to update our questionReq
	if formData, ok := result.(QuestionRequest); ok {
		questionReq = formData
	}

	userID, userExists := auth.UserIDFromContext(r.Context())
	if !userExists {
		log.Println("User ID not found in context")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	question := models.Question{
		Title:       questionReq.Title,
		Content:     questionReq.Content,
		UserID:      userID,
		Published:   false,
		TimeLimit:   questionReq.TimeLimit,
		MemoryLimit: questionReq.MemoryLimit,
		Tags:        questionReq.Tags,
	}
	db := database.GetDB()
	if db == nil {
		log.Println("Database connection is nil")
		http.Error(w, "Database connection error", http.StatusInternalServerError)
		return
	}

	dbResult := db.Create(&question)
	if dbResult.Error != nil {
		log.Printf("Database error: %v", dbResult.Error)
		http.Error(w, "Failed to create question", http.StatusInternalServerError)
		return
	}

	var testCases []models.TestCase
	for i := range questionReq.SampleInputs {
		if i < len(questionReq.SampleOutputs) {
			testCase := models.TestCase{
				QuestionID:     question.ID,
				Input:          questionReq.SampleInputs[i],
				ExpectedOutput: questionReq.SampleOutputs[i],
			}
			testCases = append(testCases, testCase)
		}
	}

	if len(testCases) > 0 {
		if err := db.Create(&testCases).Error; err != nil {
			log.Printf("Failed to create test cases: %v", err)
			http.Error(w, "Failed to create test cases", http.StatusInternalServerError)
			return
		}
	}

	log.Printf("Question created successfully with ID: %d", question.ID)

	// Based on content type, return appropriate response
	if utils.IsJSONRequest(r) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		if err := json.NewEncoder(w).Encode(question); err != nil {
			log.Printf("JSON encoding error: %v", err)
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		}
	} else {
		http.Redirect(w, r, fmt.Sprintf("/question/%d", question.ID), http.StatusSeeOther)
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

	formProcessor := func(r *http.Request) (any, error) {
		var formReq QuestionRequest

		formReq.Title = r.FormValue("title")
		formReq.Content = r.FormValue("content")

		// Parse time limit
		if timeLimitStr := r.FormValue("time_limit_ms"); timeLimitStr != "" {
			timeLimit, err := strconv.Atoi(timeLimitStr)
			if err != nil {
				return nil, fmt.Errorf("invalid time limit: %v", err)
			}
			formReq.TimeLimit = timeLimit
		}

		// Parse memory limit
		if memoryLimitStr := r.FormValue("memory_limit_mb"); memoryLimitStr != "" {
			memoryLimit, err := strconv.Atoi(memoryLimitStr)
			if err != nil {
				return nil, fmt.Errorf("invalid memory limit: %v", err)
			}
			formReq.MemoryLimit = memoryLimit
		}

		// Collect sample inputs and outputs
		formReq.SampleInputs = r.Form["sample_inputs[]"]
		formReq.SampleOutputs = r.Form["sample_outputs[]"]

		// Validate input and output pairs
		if len(formReq.SampleInputs) != len(formReq.SampleOutputs) {
			return nil, fmt.Errorf("number of sample inputs and outputs must match")
		}

		formReq.Tags = r.FormValue("tags")

		// Validate required fields
		if formReq.Title == "" || formReq.Content == "" {
			return nil, fmt.Errorf("title and content are required")
		}

		return formReq, nil
	}

	result, err := utils.ProcessRequestData(r, &questionReq, formProcessor)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if formData, ok := result.(QuestionRequest); ok {
		questionReq = formData
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

	// Start a transaction
	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var question models.Question
	if err := tx.First(&question, id).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			http.Error(w, "Question not found", http.StatusNotFound)
		} else {
			log.Printf("Database error: %v", err)
			http.Error(w, "Failed to retrieve question", http.StatusInternalServerError)
		}
		return
	}

	var user models.User
	if err := tx.First(&user, userID).Error; err != nil {
		tx.Rollback()
		log.Printf("Database error: %v", err)
		http.Error(w, "Failed to retrieve user", http.StatusInternalServerError)
		return
	}

	// Check permissions
	if question.UserID != userID && user.Role != models.AdminRole {
		tx.Rollback()
		if utils.IsFormRequest(r) {
			http.Redirect(w, r, fmt.Sprintf("/question/%d", question.ID), http.StatusSeeOther)
			return
		}
		http.Error(w, "Unauthorized to edit this question", http.StatusForbidden)
		return
	}

	// Update question fields
	question.Title = questionReq.Title
	question.Content = questionReq.Content
	question.TimeLimit = questionReq.TimeLimit
	question.MemoryLimit = questionReq.MemoryLimit
	question.Tags = questionReq.Tags

	// Handle publishing if the user is an admin
	if user.Role == models.AdminRole {
		// Assume form includes 'published' field; adjust as needed
		if publishedStr := r.FormValue("published"); publishedStr != "" {
			published, err := strconv.ParseBool(publishedStr)
			if err != nil {
				tx.Rollback()
				http.Error(w, "Invalid published value", http.StatusBadRequest)
				return
			}
			question.Published = published
			if published {
				now := time.Now()
				question.PublishedAt = &now
				question.PublishedBy = &user.ID
			} else {
				question.PublishedAt = nil
				question.PublishedBy = nil
			}
		}
	}

	// Save the question
	if err := tx.Save(&question).Error; err != nil {
		tx.Rollback()
		log.Printf("Database error: %v", err)
		http.Error(w, "Failed to update question", http.StatusInternalServerError)
		return
	}

	// Delete existing test cases
	if err := tx.Where("question_id = ?", question.ID).Delete(&models.TestCase{}).Error; err != nil {
		tx.Rollback()
		log.Printf("Failed to delete test cases: %v", err)
		http.Error(w, "Failed to update test cases", http.StatusInternalServerError)
		return
	}

	// Create new test cases
	var testCases []models.TestCase
	for i := range questionReq.SampleInputs {
		testCase := models.TestCase{
			QuestionID:     question.ID,
			Input:          questionReq.SampleInputs[i],
			ExpectedOutput: questionReq.SampleOutputs[i],
		}
		testCases = append(testCases, testCase)
	}

	if len(testCases) > 0 {
		if err := tx.Create(&testCases).Error; err != nil {
			tx.Rollback()
			log.Printf("Failed to create test cases: %v", err)
			http.Error(w, "Failed to create test cases", http.StatusInternalServerError)
			return
		}
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		log.Printf("Failed to commit transaction: %v", err)
		http.Error(w, "Failed to update question", http.StatusInternalServerError)
		return
	}

	if utils.IsFormRequest(r) {
		http.Redirect(w, r, fmt.Sprintf("/question/%d", question.ID), http.StatusSeeOther)
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

func publishQuestion(w http.ResponseWriter, r *http.Request) {
	log.Println("Publishing question...")
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid question ID", http.StatusBadRequest)
		return
	}

	var publishReq QuestionPublishRequest

	// Process form data using our utility function
	formProcessor := func(r *http.Request) (interface{}, error) {
		var formReq QuestionPublishRequest

		publishedStr := r.FormValue("published")
		formReq.Published = publishedStr == "true"

		return formReq, nil
	}

	result, err := utils.ProcessRequestData(r, &publishReq, formProcessor)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// If the result came from form processing, we need to update our publishReq
	if formData, ok := result.(QuestionPublishRequest); ok {
		publishReq = formData
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

	var user models.User
	dbResult := db.First(&user, userID)
	if dbResult.Error != nil {
		log.Printf("Database error: %v", dbResult.Error)
		http.Error(w, "Failed to retrieve user", http.StatusInternalServerError)
		return
	}

	if user.Role != models.AdminRole {
		http.Error(w, "Only administrators can publish or unpublish questions", http.StatusForbidden)
		return
	}

	var question models.Question
	dbResult = db.First(&question, id)
	if dbResult.Error != nil {
		if dbResult.Error == gorm.ErrRecordNotFound {
			http.Error(w, "Question not found", http.StatusNotFound)
		} else {
			log.Printf("Database error: %v", dbResult.Error)
			http.Error(w, "Failed to retrieve question", http.StatusInternalServerError)
		}
		return
	}

	if question.Published == publishReq.Published {
		errorMsg := "Question is already in the requested publish state"
		if utils.IsFormRequest(r) {
			var state string
			if publishReq.Published {
				state = "published"
			} else {
				state = "unpublished"
			}
			http.Redirect(w, r, fmt.Sprintf("/questions/%d?error=already_%s", id, state), http.StatusSeeOther)
			return
		}
		http.Error(w, errorMsg, http.StatusBadRequest)
		return
	}

	question.Published = publishReq.Published
	if publishReq.Published {
		publishedByID := userID
		question.PublishedBy = &publishedByID
		now := time.Now()
		question.PublishedAt = &now
	} else {
		question.PublishedBy = nil
		question.PublishedAt = nil
	}

	dbResult = db.Save(&question)
	if dbResult.Error != nil {
		log.Printf("Database error: %v", dbResult.Error)
		http.Error(w, "Failed to update question", http.StatusInternalServerError)
		return
	}

	if utils.IsFormRequest(r) {
		var successAction string
		if publishReq.Published {
			successAction = "published"
		} else {
			successAction = "unpublished"
		}
		http.Redirect(w, r, fmt.Sprintf("/question/%d?success=%s", id, successAction), http.StatusSeeOther)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(question); err != nil {
		log.Printf("JSON encoding error: %v", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func getTestCasesByQuestionID(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	questionID, err := strconv.Atoi(vars["id"])
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

	var testCases []models.TestCase
	result := db.Where("question_id = ?", questionID).Find(&testCases)
	if result.Error != nil {
		log.Printf("Database error: %v", result.Error)
		http.Error(w, "Failed to retrieve test cases", http.StatusInternalServerError)
		return
	}

	if len(testCases) == 0 {
		http.Error(w, "No test cases found for this question", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(testCases); err != nil {
		log.Printf("JSON encoding error: %v", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}
