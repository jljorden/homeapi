package links

import (
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"time"
)

const contentTypeJSON = "application/json"

type linkwardenLinksResponse struct {
	Response json.RawMessage `json:"response"`
}

var client = &http.Client{
	Timeout: 10 * time.Second,
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	},
}

func RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/links", getLinks)
}

func getLinks(w http.ResponseWriter, r *http.Request) {
	apiKey := os.Getenv("LINKWARDEN_API_KEY")

	linksAPIURL := url.URL{
		Scheme: "https",
		Host:   "jljorden.com:10443",
		Path:   "/api/v1/links",
	}

	query := linksAPIURL.Query()
	query.Set("collectionId", "1")
	linksAPIURL.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(
		r.Context(),
		http.MethodGet,
		linksAPIURL.String(),
		nil,
	)
	if err != nil {
		http.Error(
			w,
			`{"error":"failed to create links request"}`,
			http.StatusInternalServerError,
		)
		return
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", contentTypeJSON)

	resp, err := client.Do(req)
	if err != nil {
		http.Error(
			w,
			`{"error":"links service is unavailable"}`,
			http.StatusBadGateway,
		)
		return
	}
	defer resp.Body.Close()

	// Preserve Linkwarden errors instead of trying to normalize them.
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		w.Header().Set("Content-Type", contentTypeJSON)
		w.WriteHeader(resp.StatusCode)

		var apiError json.RawMessage
		if err := json.NewDecoder(resp.Body).Decode(&apiError); err == nil {
			_, _ = w.Write(apiError)
			return
		}

		_, _ = w.Write([]byte(`{"error":"Linkwarden returned an error"}`))
		return
	}

	var payload linkwardenLinksResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		http.Error(
			w,
			`{"error":"invalid response from Linkwarden"}`,
			http.StatusBadGateway,
		)
		return
	}

	if len(payload.Response) == 0 || string(payload.Response) == "null" {
		payload.Response = json.RawMessage("[]")
	}

	w.Header().Set("Content-Type", contentTypeJSON)

	if err := json.NewEncoder(w).Encode(payload.Response); err != nil {
		http.Error(
			w,
			`{"error":"failed to write links response"}`,
			http.StatusInternalServerError,
		)
	}
}
