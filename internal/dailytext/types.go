package dailytext

import (
	"time"
)

type scriptureTip struct {
	ID        string `json:"id"`
	Citation  string `json:"citation"`
	Text      string `json:"text"`
	Reference bool   `json:"reference"`
}

type response struct {
	HTML          string         `json:"html"`
	ScriptureTips []scriptureTip `json:"scriptureTips"`
	Source        string         `json:"source"`
	Cached        bool           `json:"cached"`
	Stale         bool           `json:"stale"`
	CachedAt      string         `json:"cachedAt,omitempty"`
	EntryDate     string         `json:"entryDate"`
}


type scriptureLink struct {
	ID   string
	Path string
}

type errorResponse struct {
	Error string `json:"error"`
}

type dailyTextRow struct {
	ID          int64
	EntryDate   time.Time
	ContentHTML string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
