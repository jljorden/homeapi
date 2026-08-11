package jw

import (
	"io"
	"encoding/json"
	"net/http"
)

const contentTypeJSON = "application/json"

func RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/meetings", listMeetings)
	mux.HandleFunc("GET /api/jwnews", listJWNews)
	mux.HandleFunc("GET /api/meetings/{id}", getMeeting)
}

func listMeetings(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", contentTypeJSON)
	json.NewEncoder(w).Encode(map[string]string{"message": "list meetings"})
}

func listJWNews(w http.ResponseWriter, r *http.Request) {
	url := "https://www.jw.org/en/news/rss/FullNewsRSS/feed.xml"

	resp, err := http.Get(url)
	if err != nil {
		http.Error(w, "failed to fetch RSS", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Propagate status from upstream if you want
	if resp.StatusCode != http.StatusOK {
		http.Error(w, "upstream error", http.StatusBadGateway)
		return
	}

	// Set appropriate content type for RSS/XML
	w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")

	// Stream the body as‑is
	if _, err := io.Copy(w, resp.Body); err != nil {
		// Client disconnected or write error; nothing more to do
		return
	}
}

func getMeeting(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	w.Header().Set("Content-Type", contentTypeJSON)
	json.NewEncoder(w).Encode(map[string]string{"message": "get meeting", "id": id})
}
