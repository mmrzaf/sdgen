package web

import (
	"embed"
	"net/http"
)

//go:embed templates/*.html
var templatesFS embed.FS

func IndexHandler(w http.ResponseWriter, r *http.Request) {
	data, err := templatesFS.ReadFile("templates/index.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
}

func TargetsHandler(w http.ResponseWriter, r *http.Request) {
	data, err := templatesFS.ReadFile("templates/targets.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
}

func RunDetailHandler(w http.ResponseWriter, r *http.Request) {
	data, err := templatesFS.ReadFile("templates/run.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
}
