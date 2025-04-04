package handler

import (
	"fmt"
	"html/template"
	"net/http"

	"github.com/gorilla/mux"
	// "strconv"
)

type QuestionData struct {
	Question Question
}

func QuestionHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	fmt.Println("Question ID:", id)
	// pageStr := r.URL.Query().Get("questionID")
	question := Question{Name: "asd"}

	data := QuestionData{
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
