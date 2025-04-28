package handler

import (
	"goera/serve/internal/auth"
	"html/template"
	"net/http"
)

type QuestionCreateData struct {
	ErrorMessage  string
	CurrentUserID uint // Added for dynamic profile link
}

func QuestionCreateHandler(w http.ResponseWriter, r *http.Request) {
	currentUserID, exists := auth.UserIDFromContext(r.Context())
	if !exists {
		// Redirect to login if not authenticated, as this page requires login
		http.Redirect(w, r, "/login?error=unauthorized", http.StatusSeeOther)
		return
	}

	data := QuestionCreateData{
		ErrorMessage:  r.URL.Query().Get("error"),
		CurrentUserID: currentUserID, // Populate the new field
	}

	tmpl, err := template.ParseFiles("web/templates/questionCreatorForm.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = tmpl.Execute(w, data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
