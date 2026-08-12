package greetings

import (
	"encoding/json"
	"net/http"
)

type Handler struct {
	store *Store
}

func NewHandler(store *Store) *Handler {
	return &Handler{store: store}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /goapi/greetings/random", h.RandomGreeting)
}

func (h *Handler) RandomGreeting(w http.ResponseWriter, r *http.Request) {
	period := r.URL.Query().Get("period")
	if period == "" {
		http.Error(w, `{"error":"Missing period parameter (morning, afternoon, evening)"}`, http.StatusBadRequest)
		return
	}

	g, err := h.store.GetRandomByPeriod(r.Context(), period)
	if err != nil {
		http.Error(w, `{"error":"failed to fetch greeting"}`, http.StatusInternalServerError)
		return
	}
	if g == nil {
		http.Error(w, `{"error":"no greeting found for that period"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(g)
}
