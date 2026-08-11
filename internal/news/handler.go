package news

import (
	"io"
	"net/http"
	"net/url"
	"time"
	"os"
)

var client = &http.Client{
	Timeout: 10 * time.Second,
}

func RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/news", getNews)
}

func getNews(w http.ResponseWriter, r *http.Request) {
	apiKey := os.Getenv("NEWS_API_KEY")
	if apiKey == "" {
		http.Error(w, `{"error":"Missing apikey query parameter"}`, http.StatusBadRequest)
		return
	}

	openNewsURL := url.URL{
		Scheme: "https",
		Host:   "newsapi.org",
		Path:   "/v2/top-headlines",
	}

	query := openNewsURL.Query()
	query.Set("apiKey", apiKey)
	query.Set("sources", "cnn")
	openNewsURL.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(
		r.Context(),
		http.MethodGet,
		openNewsURL.String(),
		nil,
	)
	if err != nil {
		http.Error(w, `{"error":"failed to create news request"}`, http.StatusInternalServerError)
		return
	}

	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, `{"error":"news service is unavailable"}`, http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)

	_, _ = io.Copy(w, resp.Body)
}