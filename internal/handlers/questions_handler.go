package handler

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/computer-technology-4022/goera/internal/models"
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
	// Get page from query parameters
	pageStr := r.URL.Query().Get("page")
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}

	// Create API request URL with pagination
	// Use the same host and scheme as the original request
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	host := r.Host
	apiURL := fmt.Sprintf("%s://%s/api/questions?page=%d", scheme, host, page)

	// Make HTTP request to internal API
	client := &http.Client{}
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		log.Printf("Error creating request: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Copy authentication headers/cookies from original request
	for _, cookie := range r.Cookies() {
		req.AddCookie(cookie)
	}

	// Copy authorization header if present
	if authHeader := r.Header.Get("Authorization"); authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error making API request: %v", err)
		http.Error(w, "Failed to fetch questions", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("API returned non-200 status: %d", resp.StatusCode)
		http.Error(w, "Failed to fetch questions", http.StatusInternalServerError)
		return
	}

	// Read and parse response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading response body: %v", err)
		http.Error(w, "Failed to read questions data", http.StatusInternalServerError)
		return
	}

	var apiResponse APIResponse
	err = json.Unmarshal(body, &apiResponse)
	if err != nil {
		log.Printf("Error parsing API response: %v", err)
		http.Error(w, "Failed to parse questions data", http.StatusInternalServerError)
		return
	}

	// Prepare data for template
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
