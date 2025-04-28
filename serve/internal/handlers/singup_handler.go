package handler

import (
	"html/template"
	"net/http"

	"goera/serve/internal/auth"
)

type SignUpData struct {
	ErrorMessage string
}

func SignUpHandler(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("token")
	if err == nil && cookie.Value != "" {
		claims, err := auth.ValidateJWT(cookie.Value)
		if err == nil && claims.UserID > 0 {
			http.Redirect(w, r, "/questions", http.StatusSeeOther)
			return
		}
	}

	errorCode := r.URL.Query().Get("error")
	var errorMessage string

	switch errorCode {
	case "user_exists":
		errorMessage = "Username already exists. Please choose another username."
	case "missing_fields":
		errorMessage = "Please fill in all required fields."
	case "server_error":
		errorMessage = "A server error occurred. Please try again later."
	case "invalid_form":
		errorMessage = "Invalid form submission. Please try again."
	case "":
	default:
		errorMessage = "An error occurred. Please try again."
	}

	data := SignUpData{
		ErrorMessage: errorMessage,
	}

	tmpl, err := template.ParseFiles("web/templates/signup.html")
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
