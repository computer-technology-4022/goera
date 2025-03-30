package handler

import (
	"html/template"
	"net/http"
	"strconv"
	"time"
)

type SubmissionStatus int

const (
	Pending      SubmissionStatus = iota
	OK                            // 1
	CompileError                  // 2
	WrongAnswer                   // 3
	MemoryLimit                   // 4
	TimeLimit                     // 5
	RuntimeError                  // 6
)

type Submission struct {
	QuestionName   string
	SubmissionDate time.Time
	Status         SubmissionStatus
}

type SubmissionsPageData struct {
	Submissions []Submission
	Page        int
	TotalPages  int
}

func SubmissionPageHandler(w http.ResponseWriter, r *http.Request) {
	// Pagination setup
	pageStr := r.URL.Query().Get("page")
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}

	// Mock data - replace with database query
	allSubmissions := []Submission{
		{QuestionName: "Two Sum", SubmissionDate: time.Now(), Status: OK},
		{QuestionName: "Reverse String", SubmissionDate: time.Now().Add(-1 * time.Hour), Status: Pending},
		{QuestionName: "Palindrome", SubmissionDate: time.Now().Add(-2 * time.Hour), Status: WrongAnswer},
		{QuestionName: "Tree Traversal", SubmissionDate: time.Now().Add(-3 * time.Hour), Status: TimeLimit},
		{QuestionName: "Two Sum", SubmissionDate: time.Now(), Status: OK},
		{QuestionName: "Reverse String", SubmissionDate: time.Now().Add(-1 * time.Hour), Status: Pending},
		{QuestionName: "Palindrome", SubmissionDate: time.Now().Add(-2 * time.Hour), Status: WrongAnswer},
		{QuestionName: "Tree Traversal", SubmissionDate: time.Now().Add(-3 * time.Hour), Status: TimeLimit},
		{QuestionName: "Two Sum", SubmissionDate: time.Now(), Status: OK},
		{QuestionName: "Reverse String", SubmissionDate: time.Now().Add(-1 * time.Hour), Status: Pending},
		{QuestionName: "Palindrome", SubmissionDate: time.Now().Add(-2 * time.Hour), Status: WrongAnswer},
		{QuestionName: "Tree Traversal", SubmissionDate: time.Now().Add(-3 * time.Hour), Status: TimeLimit},
	}

	// Pagination calculations
	submissionsPerPage := 10
	totalPages := (len(allSubmissions) + submissionsPerPage - 1) / submissionsPerPage
	start := (page - 1) * submissionsPerPage
	end := start + submissionsPerPage
	if end > len(allSubmissions) {
		end = len(allSubmissions)
	}

	paginatedSubmissions := allSubmissions[start:end]

	data := SubmissionsPageData{
		Submissions: paginatedSubmissions,
		Page:        page,
		TotalPages:  totalPages,
	}

	// Template functions
	funcMap := template.FuncMap{
		"sub": func(a, b int) int { return a - b },
		"add": func(a, b int) int { return a + b },
		"statusToString": func(s SubmissionStatus) string {
			switch s {
			case Pending:
				return "Pending"
			case OK:
				return "OK"
			case CompileError:
				return "Compile Error"
			case WrongAnswer:
				return "Wrong Answer"
			case MemoryLimit:
				return "Memory Limit"
			case TimeLimit:
				return "Time Limit"
			case RuntimeError:
				return "Runtime Error"
			default:
				return "Unknown"
			}
		},
		"statusToClass": func(s SubmissionStatus) string {
			switch s {
			case Pending:
				return "pending"
			case OK:
				return "ok"
			case CompileError:
				return "compile-error"
			case WrongAnswer:
				return "wrong-answer"
			case MemoryLimit:
				return "memory-limit"
			case TimeLimit:
				return "time-limit"
			case RuntimeError:
				return "runtime-error"
			default:
				return "unknown"
			}
		},
	}

	// Template execution
	tmpl := template.Must(template.New("submissionPage.html").Funcs(funcMap).ParseFiles(
		"web/templates/submissionPage.html",
	))

	err = tmpl.Execute(w, data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
