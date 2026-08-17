package meetings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/jljorden/homeapi/internal/scripture"
	"github.com/redis/go-redis/v9"
)

const (
	wolBaseURL      = "https://wol.jw.org"
	scheduleURLMask = wolBaseURL + "/en/wol/meetings/r1/lp-e/%d/%d"
)

type scriptureLink struct {
	ID   string
	Path string
}

type Service struct {
	client     *http.Client
	scriptures *scripture.Service
	redis      *redis.Client
}

func NewService(
	scriptures *scripture.Service,
	redisClient *redis.Client,
) *Service {
	return &Service{
		client: &http.Client{
			Timeout: 25 * time.Second,
		},
		scriptures: scriptures,
		redis:      redisClient,
	}
}

func (s *Service) Get(
	ctx context.Context,
	year int,
	week int,
) (Response, error) {
	cacheKey := meetingCacheKey(year, week)

	if cached, ok, err := s.getCache(ctx, cacheKey); err == nil && ok {
		return cached, nil
	}

	scheduleHTML, err := s.fetchSchedule(ctx, year, week)
	if err != nil {
		return Response{}, err
	}

	scheduleDoc, err := goquery.NewDocumentFromReader(
		strings.NewReader(scheduleHTML),
	)
	if err != nil {
		return Response{}, fmt.Errorf("parse meetings page: %w", err)
	}

	midweekPath, err := findMidweekPath(scheduleDoc)
	if err != nil {
		return Response{}, err
	}

	watchtowerPath, err := findWatchtowerPath(scheduleDoc)
	if err != nil {
		return Response{}, err
	}

	midweekPayload, err := s.scriptures.Get(ctx, midweekPath)
	if err != nil {
		return Response{}, fmt.Errorf("get midweek content: %w", err)
	}

	watchtowerPayload, err := s.scriptures.Get(ctx, watchtowerPath)
	if err != nil {
		return Response{}, fmt.Errorf("get Watchtower content: %w", err)
	}

	midweekDoc, err := documentFromFragment(midweekPayload.Content)
	if err != nil {
		return Response{}, fmt.Errorf("parse midweek HTML: %w", err)
	}

	watchtowerDoc, err := documentFromFragment(watchtowerPayload.Content)
	if err != nil {
		return Response{}, fmt.Errorf("parse Watchtower HTML: %w", err)
	}

	bibleStudyPaths, err := findBibleStudyPaths(midweekDoc)
	if err != nil {
		return Response{}, err
	}

	processContent(midweekDoc, "sc-mw")
	processContent(watchtowerDoc, "sc-wt")

	links := make([]scriptureLink, 0)
	links = append(links, collectScriptureLinks(midweekDoc, "sc-mw")...)
	links = append(links, collectScriptureLinks(watchtowerDoc, "sc-wt")...)

	var bibleStudyHTML strings.Builder

	for index, studyPath := range bibleStudyPaths {
		studyPayload, err := s.scriptures.Get(ctx, studyPath)
		if err != nil {
			continue
		}

		studyHTML := studyPayload.Content
		if len(studyPayload.Items) > 0 {
			studyHTML = studyPayload.Items[0].Content
		}

		studyDoc, err := documentFromFragment(studyHTML)
		if err != nil {
			continue
		}

		prefix := fmt.Sprintf("sc-bs%d", index)

		processContent(studyDoc, prefix)
		links = append(links, collectScriptureLinks(studyDoc, prefix)...)

		rendered, err := bodyHTML(studyDoc)
		if err != nil {
			return Response{}, fmt.Errorf("render Bible Study HTML: %w", err)
		}

		bibleStudyHTML.WriteString(rendered)
	}

	midweekHTML, err := bodyHTML(midweekDoc)
	if err != nil {
		return Response{}, fmt.Errorf("render midweek HTML: %w", err)
	}

	watchtowerHTML, err := bodyHTML(watchtowerDoc)
	if err != nil {
		return Response{}, fmt.Errorf("render Watchtower HTML: %w", err)
	}

	result := Response{
		Year:           year,
		Week:           week,
		MeetingsHTML:   midweekHTML,
		WatchtowerHTML: watchtowerHTML,
		BibleStudyHTML: "<div>" + bibleStudyHTML.String() + "</div>",
		ScriptureTips:  s.fetchTips(ctx, links),
		CachedAt:       time.Now().UTC(),
	}

	if err := s.setCache(ctx, cacheKey, result); err != nil {
		// Do not fail the request just because caching failed.
		// Add your logger here if you have one.
	}

	return result, nil
}

func (s *Service) fetchSchedule(
	ctx context.Context,
	year int,
	week int,
) (string, error) {
	requestURL := fmt.Sprintf(scheduleURLMask, year, week)

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		requestURL,
		nil,
	)
	if err != nil {
		return "", fmt.Errorf("create schedule request: %w", err)
	}

	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("User-Agent", "homeapi/1.0")

	res, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request schedule: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("schedule returned %s", res.Status)
	}

	body, err := io.ReadAll(io.LimitReader(res.Body, 10<<20))
	if err != nil {
		return "", fmt.Errorf("read schedule response: %w", err)
	}

	return string(body), nil
}

func findMidweekPath(doc *goquery.Document) (string, error) {
	var href string

	doc.Find("#materialNav h2").EachWithBreak(
		func(_ int, h2 *goquery.Selection) bool {
			if strings.TrimSpace(h2.Text()) != "Life and Ministry" {
				return true
			}

			href, _ = h2.
				NextFiltered("ul.directory.navCard").
				Find("a[href]").
				First().
				Attr("href")

			return false
		},
	)

	if href == "" {
		return "", errors.New("midweek link not found")
	}

	return scripture.NormalizePath(href)
}

func findWatchtowerPath(doc *goquery.Document) (string, error) {
	var href string

	doc.Find("#materialNav h2").EachWithBreak(
		func(_ int, h2 *goquery.Selection) bool {
			if strings.TrimSpace(h2.Text()) != "Watchtower Study" {
				return true
			}

			href, _ = h2.
				NextFiltered("ul.directory.navCard").
				Find("a[href]").
				First().
				Attr("href")

			return false
		},
	)

	if href == "" {
		return "", errors.New("Watchtower study link not found")
	}

	return scripture.NormalizePath(href)
}

func findBibleStudyPaths(doc *goquery.Document) ([]string, error) {
	paths := make([]string, 0)

	doc.Find("h3").Each(func(_ int, h3 *goquery.Selection) {
		if !strings.Contains(
			strings.TrimSpace(h3.Text()),
			"Congregation Bible Study",
		) {
			return
		}

		h3.NextFiltered("div").First().
			Find("a[href]").
			Each(func(_ int, anchor *goquery.Selection) {
				href, ok := anchor.Attr("href")
				if !ok || href == "" {
					return
				}

				cleanPath, err := scripture.NormalizePath(href)
				if err == nil {
					paths = append(paths, cleanPath)
				}
			})
	})

	if len(paths) == 0 {
		return nil, errors.New("Congregation Bible Study links not found")
	}

	return paths, nil
}

func documentFromFragment(fragment string) (*goquery.Document, error) {
	return goquery.NewDocumentFromReader(
		strings.NewReader("<html><body>" + fragment + "</body></html>"),
	)
}

func bodyHTML(doc *goquery.Document) (string, error) {
	return doc.Find("body").Html()
}

func processContent(doc *goquery.Document, prefix string) {
	doc.Find("a").Each(func(index int, anchor *goquery.Selection) {
		if href, ok := anchor.Attr("href"); ok {
			anchor.SetAttr("href", absoluteWOLURL(href))
		}

		anchor.SetAttr("id", fmt.Sprintf("%s%d", prefix, index))
		anchor.SetAttr("data", "replace")
	})

	doc.Find("img").Each(func(_ int, image *goquery.Selection) {
		if src, ok := image.Attr("src"); ok {
			image.SetAttr("src", absoluteWOLURL(src))
		}
	})

	doc.Find("textarea").Remove()
}

func collectScriptureLinks(
	doc *goquery.Document,
	prefix string,
) []scriptureLink {
	links := make([]scriptureLink, 0)

	doc.Find("a").Each(func(index int, anchor *goquery.Selection) {
		href, ok := anchor.Attr("href")
		if !ok {
			return
		}

		className, _ := anchor.Attr("class")

		if strings.Contains(className, "noTooltips") ||
			strings.Contains(href, "finder") ||
			strings.Contains(href, "data-video") ||
			strings.HasPrefix(href, "#") {
			return
		}

		cleanPath, err := scripture.NormalizePath(href)
		if err != nil {
			return
		}

		links = append(links, scriptureLink{
			ID:   fmt.Sprintf("%s%d", prefix, index),
			Path: cleanPath,
		})
	})

	return links
}

func (s *Service) fetchTips(
	ctx context.Context,
	links []scriptureLink,
) []Tip {
	const workerCount = 8

	jobs := make(chan scriptureLink)
	results := make(chan Tip)

	var wg sync.WaitGroup

	for range workerCount {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for link := range jobs {
				payload, err := s.scriptures.Get(ctx, link.Path)
				if err != nil || len(payload.Items) == 0 {
					continue
				}

				item := payload.Items[0]

				results <- Tip{
					ID:        link.ID,
					Citation:  item.Title,
					Text:      item.Content,
					Reference: item.Did != "",
				}
			}
		}()
	}

	go func() {
		for _, link := range links {
			jobs <- link
		}

		close(jobs)
		wg.Wait()
		close(results)
	}()

	tips := make([]Tip, 0, len(links))
	seen := make(map[string]struct{})

	for tip := range results {
		if _, exists := seen[tip.ID]; exists {
			continue
		}

		seen[tip.ID] = struct{}{}
		tips = append(tips, tip)
	}

	return tips
}

func absoluteWOLURL(raw string) string {
	raw = strings.TrimSpace(raw)

	if raw == "" ||
		strings.HasPrefix(raw, "#") ||
		strings.HasPrefix(raw, "mailto:") ||
		strings.HasPrefix(raw, "tel:") ||
		strings.HasPrefix(raw, "data:") ||
		strings.HasPrefix(raw, "http://") ||
		strings.HasPrefix(raw, "https://") {
		return raw
	}

	return wolBaseURL + "/" + strings.TrimLeft(raw, "/")
}

func meetingCacheKey(year int, week int) string {
	return fmt.Sprintf("homeapi:meetings:%d:%02d", year, week)
}

func (s *Service) getCache(
	ctx context.Context,
	key string,
) (Response, bool, error) {
	value, err := s.redis.Get(ctx, key).Bytes()

	if err == redis.Nil {
		return Response{}, false, nil
	}

	if err != nil {
		return Response{}, false, err
	}

	var response Response

	if err := json.Unmarshal(value, &response); err != nil {
		// Bad/stale data should not keep breaking requests.
		_ = s.redis.Del(ctx, key).Err()
		return Response{}, false, err
	}

	return response, true, nil
}

func (s *Service) setCache(
	ctx context.Context,
	key string,
	response Response,
) error {
	value, err := json.Marshal(response)
	if err != nil {
		return err
	}

	return s.redis.Set(
		ctx,
		key,
		value,
		ttlUntilNextMonday(),
	).Err()
}

func ttlUntilNextMonday() time.Duration {
	now := time.Now()

	daysUntilMonday := (int(time.Monday) - int(now.Weekday()) + 7) % 7
	if daysUntilMonday == 0 {
		daysUntilMonday = 7
	}

	nextMonday := now.AddDate(0, 0, daysUntilMonday)

	expiresAt := time.Date(
		nextMonday.Year(),
		nextMonday.Month(),
		nextMonday.Day(),
		0,
		0,
		0,
		0,
		now.Location(),
	)

	return time.Until(expiresAt)
}

func nextMondayUTC() time.Time {
	now := time.Now().UTC()

	// In Go: Sunday = 0, Monday = 1, ..., Saturday = 6.
	daysUntilMonday := (int(time.Monday) - int(now.Weekday()) + 7) % 7

	// If it is currently Monday, expire at next Monday—not immediately.
	if daysUntilMonday == 0 {
		daysUntilMonday = 7
	}

	nextMonday := now.AddDate(0, 0, daysUntilMonday)

	return time.Date(
		nextMonday.Year(),
		nextMonday.Month(),
		nextMonday.Day(),
		0,
		0,
		0,
		0,
		time.UTC,
	)
}
