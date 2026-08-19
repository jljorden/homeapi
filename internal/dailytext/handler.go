package dailytext

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"time"
)

type handler struct {
	db *sql.DB
}

func RegisterRoutes(mux *http.ServeMux, db *sql.DB) {
	h := &handler{
		db: db,
	}

	mux.HandleFunc(
		"GET /goapi/dailytext",
		h.getDailyText,
	)
}

func (h *handler) getDailyText(
	w http.ResponseWriter,
	r *http.Request,
) {
	dateText := r.URL.Query().Get("date")

	if !validDate(dateText) {
		writeJSON(w, http.StatusBadRequest, errorResponse{
			Error: "date must be YYYY-MM-DD",
		})
		return
	}

	entryDate, err := time.Parse("2006-01-02", dateText)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{
			Error: "date must be YYYY-MM-DD",
		})
		return
	}

	entry, err := h.getFromDatabase(r.Context(), entryDate)
	if errors.Is(err, sql.ErrNoRows) {
		writeJSON(w, http.StatusNotFound, errorResponse{
			Error: "daily text not found",
		})
		return
	}

	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{
			Error: "could not retrieve daily text",
		})
		return
	}

	html, tips, err := h.processHTML(
		r.Context(),
		entry.ContentHTML,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{
			Error: "could not process daily text",
		})
		return
	}

	if tips == nil {
		tips = make([]scriptureTip, 0)
	}

	writeJSON(w, http.StatusOK, response{
		HTML:          html,
		ScriptureTips: tips,
		Source:        "postgresql:daily_texts",
		Cached:        false,
		Stale:         false,
		CachedAt:      entry.UpdatedAt.UTC().Format(time.RFC3339),
		EntryDate:     entry.EntryDate.Format("2006-01-02"),
	})
}

func (h *handler) getFromDatabase(
	ctx context.Context,
	entryDate time.Time,
) (dailyTextRow, error) {
	const query = `
		SELECT
			id,
			entry_date,
			content_html,
			created_at,
			updated_at
		FROM daily_texts
		WHERE entry_date = $1
	`

	var entry dailyTextRow

	err := h.db.QueryRowContext(
		ctx,
		query,
		entryDate,
	).Scan(
		&entry.ID,
		&entry.EntryDate,
		&entry.ContentHTML,
		&entry.CreatedAt,
		&entry.UpdatedAt,
	)
	if err != nil {
		return dailyTextRow{}, err
	}

	return entry, nil
}

func validDate(value string) bool {
	date, err := time.Parse("2006-01-02", value)

	return err == nil &&
		date.Format("2006-01-02") == value
}

func writeJSON(
	w http.ResponseWriter,
	status int,
	payload any,
) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(payload)
}