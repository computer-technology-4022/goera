package handler

import (
	"html/template"
	"net/http"
	"strconv"
)

type QuestionsData struct {
	Questions  []Question
	Page       int
	TotalPages int
}

type Question struct {
	Name string
}

func QuestionsHandler(w http.ResponseWriter, r *http.Request) {
	pageStr := r.URL.Query().Get("page")
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1 // Default to page 1 if invalid or missing
	}
	questions := []Question{
		{Name: "abass"},
		{Name: "javad"},
		{Name: "hosein"},
		{Name: "javad"},
		{Name: "koaf"},
		{Name: "123"},
		{Name: "asjd"},
	}

	questionsPerPage := 3
	totalPages := (len(questions) + questionsPerPage - 1) / questionsPerPage
	start := (page - 1) * questionsPerPage
	end := start + questionsPerPage
	if end > len(questions) {
		end = len(questions)
	}

	finalQuestions := questions[start:end]

	data := QuestionsData{
		Questions:  finalQuestions,
		TotalPages: totalPages,
		Page:       page,
	}

	funcMap := template.FuncMap{
		"sub": func(a, b int) int { return a - b },
		"add": func(a, b int) int { return a + b },
	}

	tmpl := template.Must(template.New("questions.html").
		Funcs(funcMap).ParseFiles("../web/templates/questions.html"))

	err = tmpl.Execute(w, data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
