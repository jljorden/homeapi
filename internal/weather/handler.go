package weather

import (
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	latitude  = "33.4936"
	longitude = "-111.9167"
)

var client = &http.Client{
	Timeout: 10 * time.Second,
}

func RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /goapi/weather", getWeather)
}

func getWeather(w http.ResponseWriter, r *http.Request) {
	apiKey := r.URL.Query().Get("apikey")
	if apiKey == "" {
		http.Error(w, `{"error":"Missing apikey query parameter"}`, http.StatusBadRequest)
		return
	}

	openWeatherURL := url.URL{
		Scheme: "https",
		Host:   "api.openweathermap.org",
		Path:   "/data/3.0/onecall",
	}

	query := openWeatherURL.Query()
	query.Set("lat", latitude)
	query.Set("lon", longitude)
	query.Set("units", "imperial")
	query.Set("appid", apiKey)
	openWeatherURL.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(
		r.Context(),
		http.MethodGet,
		openWeatherURL.String(),
		nil,
	)
	if err != nil {
		http.Error(w, `{"error":"failed to create weather request"}`, http.StatusInternalServerError)
		return
	}

	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, `{"error":"weather service is unavailable"}`, http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)

	_, _ = io.Copy(w, resp.Body)
}
