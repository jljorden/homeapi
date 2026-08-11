package router

import (
	"database/sql"
	"net/http"

	"github.com/jljorden/homeapi/internal/greetings"
	"github.com/jljorden/homeapi/internal/meetings"
	"github.com/jljorden/homeapi/internal/weather"
	"github.com/jljorden/homeapi/internal/dns"
	"github.com/jljorden/homeapi/internal/links"
	"github.com/jljorden/homeapi/internal/nut"	
)

func New(db *sql.DB) *http.ServeMux {
	mux := http.NewServeMux()

	store := greetings.NewStore(db)
	greetings.NewHandler(store).RegisterRoutes(mux)

	weather.RegisterRoutes(mux)
	meetings.RegisterRoutes(mux)
	links.RegisterRoutes(mux)
	dns.RegisterRoutes(mux)
	nut.RegisterRoutes(mux)
	return mux
}