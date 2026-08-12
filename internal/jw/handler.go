package jw

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"time"
)

const contentTypeJSON = "application/json"

var client = &http.Client{
	Timeout: 10 * time.Second,
}

func RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /goapi/meetings", listMeetings)
	mux.HandleFunc("GET /goapi/jwnews", listJWNews)
	mux.HandleFunc("GET /goapi/newsscripture", getNewsScripture)
	mux.HandleFunc("GET /goapi/meetings/{id}", getMeeting)
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

func getNewsScripture(w http.ResponseWriter, r *http.Request) {
	verses := r.URL.Query().Get("verses")
	if verses == "" {
		http.Error(w, `{"error":"Missing verses query parameter"}`, http.StatusBadRequest)
		return
	}

	scriptureURL := url.URL{
		Scheme: "https",
		Host:   "www.jw.org",
		Path:   "/en/library/bible/study-bible/books/json/html/" + verses,
	}

	query := scriptureURL.Query()
	scriptureURL.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(
		r.Context(),
		http.MethodGet,
		scriptureURL.String(),
		nil,
	)
	if err != nil {
		http.Error(w, `{"error":"failed to create scripture request"}`, http.StatusInternalServerError)
		return
	}

	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, `{"error":"scripture service is unavailable"}`, http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Propagate status from upstream if you want
	if resp.StatusCode != http.StatusOK {
		http.Error(w, "upstream error", http.StatusBadGateway)
		return
	}

	// Set appropriate content type for RSS/XML
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)

	_, _ = io.Copy(w, resp.Body)
}

func getMeeting(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	w.Header().Set("Content-Type", contentTypeJSON)
	json.NewEncoder(w).Encode(map[string]string{"message": "get meeting", "id": id})
}
