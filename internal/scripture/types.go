package scripture

import "time"

type Item struct {
	Title   string `json:"title"`
	Content string `json:"content"`
	Did     string `json:"did,omitempty"`
}

type Response struct {
	Content   string    `json:"content,omitempty"`
	Items     []Item    `json:"items,omitempty"`
	CachedAt  time.Time `json:"cachedAt"`
}