package handler

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"

	"github.com/computer-technology-4022/goera/internal/models"
	"github.com/computer-technology-4022/goera/internal/utils"
)

type QuestionsData struct {
	Questions  []models.Question
	Page       int
	PageSize   int
	TotalItems int64
	TotalPages int
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

	data := QuestionsData{
		Questions:  apiResponse.Data,
		Page:       apiResponse.Page,
		PageSize:   apiResponse.PageSize,
		TotalItems: apiResponse.TotalItems,
		TotalPages: apiResponse.TotalPages,
	}


	funcMap := template.FuncMap{
		"sub": func(a, b int) int { return a - b },
		"add": func(a, b int) int { return a + b },
	}

	tmpl := template.Must(template.New("questions.html").
		Funcs(funcMap).ParseFiles("web/templates/questions.html"))

	err = tmpl.Execute(w, data)
	if err != nil {
		log.Printf("Error executing template: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
