package handler

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"

	"goera/serve/internal/auth"
	"goera/serve/internal/models"
	"goera/serve/internal/utils"
)

type QuestionsData struct {
	Questions     []models.Question
	Page          int
	PageSize      int
	TotalItems    int64
	TotalPages    int
	CurrentUserID uint
}

type APIResponse struct {
	Data       []models.Question `json:"data"`
	Page       int               `json:"page"`
	PageSize   int               `json:"page_size"`
	TotalItems int64             `json:"total_items"`
	TotalPages int               `json:"total_pages"`
}

func QuestionsHandler(w http.ResponseWriter, r *http.Request) {
	pageStr := r.URL.Query().Get("page")
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}

	apiPath := fmt.Sprintf("/api/questions?page=%d", page)
	apiClient := utils.GetAPIClient()
	var apiResponse APIResponse
	err = apiClient.Get(r, apiPath, &apiResponse)
	if err != nil {
		log.Printf("Error fetching questions: %v", err)
		http.Error(w, "Failed to fetch questions", http.StatusInternalServerError)
		return
	}

	// Get current user ID for the profile link
	currentUserID, _ := auth.UserIDFromContext(r.Context()) // Ignore error, default to 0 if not found

	data := QuestionsData{
		Questions:     apiResponse.Data,
		Page:          apiResponse.Page,
		PageSize:      apiResponse.PageSize,
		TotalItems:    apiResponse.TotalItems,
		TotalPages:    apiResponse.TotalPages,
		CurrentUserID: currentUserID, // Populate the new field
	}
	// fmt.Println(currentUserID)
	funcMap := template.FuncMap{
		"sub": func(a, b int) int { return a - b },
		"add": func(a, b int) int { return a + b },
	}

	// Create a new template, add functions, then parse the file
	tmpl, err := template.New("questions.html").Funcs(funcMap).ParseFiles("web/templates/questions.html")
	if err != nil {
		log.Printf("Error parsing questions template: %v", err)
		http.Error(w, "Internal server error (template parse)", http.StatusInternalServerError)
		return
	}

	// Execute the template
	err = tmpl.ExecuteTemplate(w, "questions.html", data) // Execute by the name provided in New()
	if err != nil {
		log.Printf("Error executing questions template: %v", err)
		// http.Error(w, err.Error(), http.StatusInternalServerError) // Avoid potentially writing headers twice
		return
	}
}
