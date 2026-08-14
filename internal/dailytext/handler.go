package dailytext

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	cacheDir       = "/app/data/daily-text-cache"
	cacheTTL       = 6 * time.Hour
	requestTimeout = 15 * time.Second
	maxHTMLBytes   = 2 << 20
)

type handler struct {
	client  *http.Client
	cacheMu sync.Mutex
}

type response struct {
	HTML     string `json:"html"`
	Source   string `json:"source"`
	Cached   bool   `json:"cached"`
	Stale    bool   `json:"stale"`
	CachedAt string `json:"cachedAt,omitempty"`
}

type errorResponse struct {
	Error string `json:"error"`
}

type cacheEntry struct {
	Date     string    `json:"date"`
	HTML     string    `json:"html"`
	Source   string    `json:"source"`
	CachedAt time.Time `json:"cachedAt"`
}

func RegisterRoutes(mux *http.ServeMux) {
	h := &handler{
		client: &http.Client{
			Timeout: requestTimeout,
		},
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
	date := r.URL.Query().Get("date")

	if !validDate(date) {
		writeJSON(w, http.StatusBadRequest, errorResponse{
			Error: "date must be YYYY/MM/DD",
		})
		return
	}

	entry, cached, stale, err := h.fetchOrCache(r.Context(), date)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{
			Error: err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, response{
		HTML:     entry.HTML,
		Source:   entry.Source,
		Cached:   cached,
		Stale:    stale,
		CachedAt: entry.CachedAt.UTC().Format(time.RFC3339),
	})
}

func (h *handler) fetchOrCache(
	parent context.Context,
	date string,
) (cacheEntry, bool, bool, error) {
	h.cacheMu.Lock()
	defer h.cacheMu.Unlock()

	cached, cacheErr := loadCache(date)

	if cacheErr == nil && time.Since(cached.CachedAt) < cacheTTL {
		return *cached, true, false, nil
	}

	sourceURL := "https://wol.jw.org/en/wol/h/r1/lp-e/" + date

	ctx, cancel := context.WithTimeout(parent, requestTimeout)
	defer cancel()

	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		sourceURL,
		nil,
	)
	if err != nil {
		return cacheEntry{}, false, false, err
	}

	request.Header.Set(
		"Accept",
		"text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
	)
	request.Header.Set("Accept-Language", "en-US,en;q=0.9")

	upstream, err := h.client.Do(request)
	if err != nil {
		if cached != nil {
			return *cached, true, true, nil
		}

		return cacheEntry{}, false, false, err
	}
	defer upstream.Body.Close()

	challenged := upstream.StatusCode == http.StatusAccepted &&
		strings.EqualFold(
			upstream.Header.Get("x-amzn-waf-action"),
			"challenge",
		)

	if challenged {
		if cached != nil {
			return *cached, true, true, nil
		}

		return cacheEntry{}, false, false, errors.New(
			"WOL challenged the request and no cached text is available",
		)
	}

	if upstream.StatusCode < 200 || upstream.StatusCode >= 300 {
		if cached != nil {
			return *cached, true, true, nil
		}

		return cacheEntry{}, false, false, fmt.Errorf(
			"WOL returned HTTP %d",
			upstream.StatusCode,
		)
	}

	body, err := io.ReadAll(io.LimitReader(upstream.Body, maxHTMLBytes))
	if err != nil {
		if cached != nil {
			return *cached, true, true, nil
		}

		return cacheEntry{}, false, false, err
	}

	fresh := cacheEntry{
		Date:     date,
		HTML:     string(body),
		Source:   sourceURL,
		CachedAt: time.Now().UTC(),
	}

	if err := saveCache(fresh); err != nil {
		fmt.Printf("daily-text cache save failed: %v\n", err)
	}

	return fresh, false, false, nil
}

func cacheFile(date string) string {
	return filepath.Join(
		cacheDir,
		strings.ReplaceAll(date, "/", "-")+".json",
	)
}

func loadCache(date string) (*cacheEntry, error) {
	body, err := os.ReadFile(cacheFile(date))
	if err != nil {
		return nil, err
	}

	var entry cacheEntry
	if err := json.Unmarshal(body, &entry); err != nil {
		return nil, err
	}

	if entry.Date != date || entry.HTML == "" || entry.Source == "" {
		return nil, errors.New("invalid daily-text cache file")
	}

	return &entry, nil
}

func saveCache(entry cacheEntry) error {
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return err
	}

	file, err := os.CreateTemp(cacheDir, "*.tmp")
	if err != nil {
		return err
	}

	tempName := file.Name()

	defer func() {
		_ = file.Close()
		_ = os.Remove(tempName)
	}()

	body, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	if _, err := file.Write(body); err != nil {
		return err
	}

	if err := file.Close(); err != nil {
		return err
	}

	return os.Rename(file.Name(), cacheFile(entry.Date))
}

func validDate(value string) bool {
	date, err := time.Parse("2006/01/02", value)
	return err == nil && date.Format("2006/01/02") == value
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