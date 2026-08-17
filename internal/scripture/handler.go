package scripture

import (
	"encoding/json"
	"net/http"
)

func (s *Service) Handler(w http.ResponseWriter, r *http.Request) {
	result, err := s.Get(r.Context(), r.PathValue("path"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
		return
	}

	w.Header().Set("Cache-Control", "public, max-age=3600, s-maxage=86400")
	writeJSON(w, http.StatusOK, result)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(value)
}