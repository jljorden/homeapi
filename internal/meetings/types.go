package meetings

import "time"

type Tip struct {
	ID        string `json:"id"`
	Citation  string `json:"citation"`
	Text      string `json:"text"`
	Reference bool   `json:"reference"`
}

type Response struct {
	Year           int       `json:"year"`
	Week           int       `json:"week"`
	MeetingsHTML   string    `json:"meetingsHtml"`
	WatchtowerHTML string    `json:"watchtowerHtml"`
	BibleStudyHTML string    `json:"bibleStudyHtml"`
	ScriptureTips  []Tip     `json:"scriptureTips"`
	CachedAt       time.Time `json:"cachedAt"`
}