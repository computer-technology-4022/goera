package handler

import (
	"html/template"
	"net/http"
	// "strconv"
)

type QuestionData struct {
	Question Question
}

func QuestionHandler(w http.ResponseWriter, r *http.Request) {
	// pageStr := r.URL.Query().Get("questionID")
	question := Question{Name: "asd"}

	data := QuestionData{
		Question: question,
	}

	funcMap := template.FuncMap{}

	tmpl := template.Must(template.New("question.html").
		Funcs(funcMap).ParseFiles("web/templates/question.html"))

	err := tmpl.Execute(w, data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
