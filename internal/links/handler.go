package links

import (
	"crypto/tls"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"
)

const contentTypeJSON = "application/json"

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
	
	req.Header.Set("Authorization", "Bearer " + apiKey)
	req.Header.Set("Accept", contentTypeJSON)
	req.Header.Set("Content-Type", contentTypeJSON)

	if err != nil {
		http.Error(w, `{"error":"failed to create links request"}`, http.StatusInternalServerError)
		return
	}

	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, `{"error":"links service is unavailable"}`, http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", contentTypeJSON)
	w.WriteHeader(resp.StatusCode)

	_, _ = io.Copy(w, resp.Body)
}
