package router

import (
	"database/sql"
	"net/http"

	"github.com/jljorden/homeapi/internal/dailytext"
	"github.com/jljorden/homeapi/internal/dns"
	"github.com/jljorden/homeapi/internal/greetings"
	"github.com/jljorden/homeapi/internal/jw"
	"github.com/jljorden/homeapi/internal/links"
	"github.com/jljorden/homeapi/internal/news"
	"github.com/jljorden/homeapi/internal/nut"
	"github.com/jljorden/homeapi/internal/randomscripture"
	"github.com/jljorden/homeapi/internal/weather"
)

func New(db *sql.DB) *http.ServeMux {
	mux := http.NewServeMux()

	store := greetings.NewStore(db)
	greetings.NewHandler(store).RegisterRoutes(mux)

	weather.RegisterRoutes(mux)
	jw.RegisterRoutes(mux)
	links.RegisterRoutes(mux)
	dns.RegisterRoutes(mux)
	nut.RegisterRoutes(mux)
	news.RegisterRoutes(mux)
	dailytext.RegisterRoutes(mux)

	scriptureHandler := randomscripture.NewScriptureHandler()

	mux.HandleFunc(
		"GET /goapi/randomscripture",
		scriptureHandler.GetRandomScripture,
	)

	return mux
}