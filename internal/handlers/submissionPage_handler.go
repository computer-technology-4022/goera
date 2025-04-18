package handler

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"

	"github.com/computer-technology-4022/goera/internal/auth"
	"github.com/computer-technology-4022/goera/internal/models"
	"github.com/computer-technology-4022/goera/internal/utils"
)

// SubmissionPageData holds the data needed for the submissions page template
type SubmissionPageData struct {
	Submissions   []models.Submission
	Page          int
	PageSize      int
	TotalItems    int64
	TotalPages    int
	CurrentUserID uint
}

// SubmissionAPIResponse matches the API's response format
type SubmissionAPIResponse struct {
	Data       []models.Submission `json:"data"`
	Page       int                 `json:"page"`
	PageSize   int                 `json:"page_size"`
	TotalItems int64               `json:"total_items"`
	TotalPages int                 `json:"total_pages"`
}

func SubmissionPageHandler(w http.ResponseWriter, r *http.Request) {
	// Pagination setup
	pageStr := r.URL.Query().Get("page")
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}

	// Fetch submissions from the API with pagination
	apiPath := fmt.Sprintf("/api/submissions?page=%d&page_size=5", page)
	apiClient := utils.GetAPIClient()
	var apiResponse SubmissionAPIResponse
	err = apiClient.Get(r, apiPath, &apiResponse)
	if err != nil {
		log.Printf("Error fetching submissions: %v", err)
		http.Error(w, "Failed to fetch submissions", http.StatusInternalServerError)
		return
	}

	// Get current user ID for the profile link
	currentUserID, _ := auth.UserIDFromContext(r.Context()) // Ignore error, default to 0 if not found

	data := SubmissionPageData{
		Submissions:   apiResponse.Data,
		Page:          apiResponse.Page,
		PageSize:      apiResponse.PageSize,
		TotalItems:    apiResponse.TotalItems,
		TotalPages:    apiResponse.TotalPages,
		CurrentUserID: currentUserID,
	}

	// Template functions
	funcMap := template.FuncMap{
		"sub": func(a, b int) int { return a - b },
		"add": func(a, b int) int { return a + b },
		"mul": func(a, b int) int { return a * b },
		"min": func(a int, b int64) int64 {
			if int64(a) < b {
				return int64(a)
			}
			return b
		},
		"statusToString": func(s models.JudgeStatus) string {
			return string(s)
		},
		"statusToClass": func(s models.JudgeStatus) string {
			switch s {
			case models.Pending:
				return "pending"
			case models.Accepted:
				return "Accepted"
			case models.CompilationError:
				return "compile-error"
			case models.Rejected:
				return "wrong-answer"
			case models.MemoryLimitExceeded:
				return "memory-limit"
			case models.TimeLimitExceeded:
				return "time-limit"
			case models.RuntimeError:
				return "runtime-error"
			default:
				return "unknown"
			}
		},
	}

	// Template execution
	tmpl, err := template.New("submissionPage.html").Funcs(funcMap).ParseFiles("web/templates/submissionPage.html")
	if err != nil {
		log.Printf("Error parsing submission template: %v", err)
		http.Error(w, "Internal server error (template parse)", http.StatusInternalServerError)
		return
	}

	err = tmpl.ExecuteTemplate(w, "submissionPage.html", data)
	if err != nil {
		log.Printf("Error executing submission template: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
