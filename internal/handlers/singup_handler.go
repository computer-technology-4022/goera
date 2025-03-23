package handler

import (
	"html/template"
	"net/http"
)

func SignUpHandler(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFiles("../web/templates/signup.html")
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
