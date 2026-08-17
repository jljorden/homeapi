package meetings

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

func (s *Service) Handler(w http.ResponseWriter, r *http.Request) {
	year, err := strconv.Atoi(r.PathValue("year"))
	if err != nil || year < 2000 || year > 2100 {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "year must be between 2000 and 2100",
		})
		return
	}

	week, err := strconv.Atoi(r.PathValue("week"))
	if err != nil || week < 1 || week > 53 {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "week must be between 1 and 53",
		})
		return
	}

	result, err := s.Get(r.Context(), year, week)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{
			"error": err.Error(),
		})
		return
	}

	expiresAt := nextMondayUTC()
	maxAge := int(time.Until(expiresAt).Seconds())

	w.Header().Set(
		"Cache-Control",
		fmt.Sprintf("public, max-age=%d, s-maxage=%d", maxAge, maxAge),
	)
	
	writeJSON(w, http.StatusOK, result)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(value)
}
