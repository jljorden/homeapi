package router

import (
	"database/sql"
	"net/http"
	"github.com/jljorden/homeapi/internal/dailytext"
	"github.com/jljorden/homeapi/internal/dns"
	"github.com/jljorden/homeapi/internal/greetings"
	"github.com/jljorden/homeapi/internal/jw"
	"github.com/jljorden/homeapi/internal/links"
	"github.com/jljorden/homeapi/internal/meetings"
	"github.com/jljorden/homeapi/internal/news"
	"github.com/jljorden/homeapi/internal/nut"
	"github.com/jljorden/homeapi/internal/randomscripture"
	"github.com/jljorden/homeapi/internal/scripture"
	"github.com/jljorden/homeapi/internal/weather"
	"os"
	"strconv"
	"github.com/redis/go-redis/v9"
)

func New(db *sql.DB) *http.ServeMux {
	redisDB, err := strconv.Atoi(os.Getenv("REDIS_DB"))
	if err != nil {
		redisDB = 0
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr: os.Getenv("REDIS_ADDR"),
		DB:   redisDB,
	})

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

	randomScriptureHandler := randomscripture.NewScriptureHandler()
	mux.HandleFunc(
		"GET /goapi/randomscripture",
		randomScriptureHandler.GetRandomScripture,
	)

	scriptureService := scripture.NewService(redisClient)
	meetingsService := meetings.NewService(
		scriptureService,
		redisClient,
	)

	mux.HandleFunc(
		"GET /goapi/scripture/{path...}",
		scriptureService.Handler,
	)

	mux.HandleFunc(
		"GET /goapi/meetings/{year}/{week}",
		meetingsService.Handler,
	)

	return mux
}
