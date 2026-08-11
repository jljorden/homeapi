package meetings

import (
	"encoding/json"
	"net/http"
)

func RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/meetings", listMeetings)
	mux.HandleFunc("GET /api/meetings/{id}", getMeeting)
}

func listMeetings(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "list meetings"})
}

func getMeeting(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "get meeting", "id": id})
}