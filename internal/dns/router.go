package dns

import (
	"crypto/tls"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"
)

var client = &http.Client{
	Timeout: 10 * time.Second,
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	},
}

func RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/dns", getDNS)
}

func getDNS(w http.ResponseWriter, r *http.Request) {
	apiKey := os.Getenv("DNS_API_KEY")

	dnsAPIURL := url.URL{
		Scheme: "https",
		Host:   "mercury.jljorden.com:53443",
		Path:   "/api/dashboard/stats/get",
	}

	query := dnsAPIURL.Query()
	query.Set("token", apiKey)
	query.Set("type", "LastHour")
	query.Set("utc", "true")
	dnsAPIURL.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(
		r.Context(),
		http.MethodGet,
		dnsAPIURL.String(),
		nil,
	)
	
	if err != nil {
		http.Error(w, `{"error":"failed to create dns request"}`, http.StatusInternalServerError)
		return
	}

	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, `{"error":"dns service is unavailable"}`, http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)

	_, _ = io.Copy(w, resp.Body)
}
