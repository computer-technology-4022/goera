package handler

import (
	"html/template"
	"net/http"
)

type LoginData struct {
	ErrorMessage string
}

func LoginHandler(w http.ResponseWriter, r *http.Request) {
	errorCode := r.URL.Query().Get("error")
	var errorMessage string

	switch errorCode {
	case "invalid_credentials":
		errorMessage = "Invalid username or password. Please try again."
	case "server_error":
		errorMessage = "A server error occurred. Please try again later."
	case "unauthorized":
		errorMessage = "Please login to access that page."
	case "":
	default:
		errorMessage = "An error occurred. Please try again."
	}

	data := LoginData{
		ErrorMessage: errorMessage,
	}

	tmpl, err := template.ParseFiles("web/templates/login.html")
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
