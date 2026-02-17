package web

import (
	"embed"
	"net/http"
)

//go:embed templates/*.html
var templates embed.FS

func IndexHandler(w http.ResponseWriter, r *http.Request) {
	data, err := templates.ReadFile("templates/index.html")
	if err != nil {
		http.Error(w, "Template not found", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html")
	w.Write(data)
}

func RunDetailHandler(w http.ResponseWriter, r *http.Request) {
	data, err := templates.ReadFile("templates/run.html")
	if err != nil {
		http.Error(w, "Template not found", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html")
	w.Write(data)
}
