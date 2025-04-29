package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"goera/serve/internal/database"
	"goera/serve/internal/models"

	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

type Result string

const (
	Accepted     Result = "Accepted"
	CompileError Result = "CompileError"
	WrongAnswer  Result = "WrongAnswer"
	MemoryLimit  Result = "MemoryLimit"
	TimeLimit    Result = "TimeLimit"
	RuntimeError Result = "RuntimeError"
)

func ServerJudgeHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		updateSubmission(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// updateSubmission updates a submission's status and results
func updateSubmission(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid submission ID", http.StatusBadRequest)
		return
	}

	// Parse request body
	var updateData struct {
		QuestionID uint               `json:"questionId"`
		Status     models.JudgeStatus `json:"status"`
		Output     string             `json:"output"`
	}

	if err := json.NewDecoder(r.Body).Decode(&updateData); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	log.Println(updateData.Status)

	db := database.GetDB()
	if db == nil {
		log.Println("Database connection is nil")
		http.Error(w, "Database connection error", http.StatusInternalServerError)
		return
	}

	// Find the submission
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

	// Update fields
	submission.JudgeStatus = updateData.Status
	submission.Error = updateData.Output

	// Save updates
	result = db.Save(&submission)
	if result.Error != nil {
		log.Printf("Database error updating submission: %v", result.Error)
		http.Error(w, "Failed to update submission", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(submission); err != nil {
		log.Printf("JSON encoding error: %v", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}
