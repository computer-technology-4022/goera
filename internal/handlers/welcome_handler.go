package handler

import (
	"html/template"
	"net/http"

	"github.com/computer-technology-4022/goera/internal/auth"
)

func WelcomeHandler(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("token")
	if err == nil && cookie.Value != "" {
		claims, err := auth.ValidateJWT(cookie.Value)
		if err == nil && claims.UserID > 0 {
			http.Redirect(w, r, "/questions", http.StatusSeeOther)
			return
		}
	}

	tmpl, err := template.ParseFiles("web/templates/index.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = tmpl.Execute(w, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
