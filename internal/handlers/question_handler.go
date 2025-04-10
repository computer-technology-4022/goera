package handler

import (
	"fmt"
	"html/template"
	"net/http"

	"github.com/computer-technology-4022/goera/internal/models"
	"github.com/gorilla/mux"
	// "strconv"
)

type QuestionPageData struct {
	Question models.Question
}

func QuestionHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	fmt.Println("Question ID:", id)
	// pageStr := r.URL.Query().Get("questionID")
	question := models.Question{Title: "kis"}

	data := QuestionPageData{
		Question: question,
	}

	funcMap := template.FuncMap{}

	tmpl := template.Must(template.New("question.html").
		Funcs(funcMap).ParseFiles("web/templates/question.html", "web/templates/base.html"))

	err := tmpl.Execute(w, data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
