package dailytext

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const wolOrigin = "https://wol.jw.org"

func (h *handler) processHTML(
	ctx context.Context,
	contentHTML string,
) (string, []scriptureTip, error) {
	doc, err := goquery.NewDocumentFromReader(
		strings.NewReader(
			`<div id="daily-text-root">` +
				contentHTML +
				`</div>`,
		),
	)
	if err != nil {
		return "", nil, fmt.Errorf("parse daily-text HTML: %w", err)
	}

	links := make([]scriptureLink, 0)

	doc.Find("#daily-text-root a[href]").Each(
		func(index int, selection *goquery.Selection) {
			href, exists := selection.Attr("href")
			if !exists || href == "" {
				return
			}

			absoluteURL, err := normalizeWOLURL(href)
			if err != nil {
				return
			}

			selection.SetAttr("href", absoluteURL)

			if !isScriptureURL(absoluteURL) {
				return
			}

			parsedURL, err := url.Parse(absoluteURL)
			if err != nil {
				return
			}

			id := fmt.Sprintf("sc%d", index)

			selection.SetAttr("id", id)
			selection.SetAttr("data-scripture-tooltip", "true")

			links = append(links, scriptureLink{
				ID:   id,
				Path: strings.TrimPrefix(parsedURL.Path, "/"),
			})
		},
	)

	tips := h.fetchScriptureTips(ctx, links)

	renderedHTML, err := doc.Find("#daily-text-root").Html()
	if err != nil {
		return "", nil, fmt.Errorf("render daily-text HTML: %w", err)
	}

	if tips == nil {
		tips = make([]scriptureTip, 0)
	}

	return renderedHTML, tips, nil
}

func normalizeWOLURL(href string) (string, error) {
	baseURL, err := url.Parse(wolOrigin)
	if err != nil {
		return "", err
	}

	referenceURL, err := url.Parse(href)
	if err != nil {
		return "", err
	}

	return baseURL.ResolveReference(referenceURL).String(), nil
}

func isScriptureURL(href string) bool {
	parsedURL, err := url.Parse(href)
	if err != nil {
		return false
	}

	return strings.HasPrefix(parsedURL.Path, "/wol/bc/")
}

func (h *handler) fetchScriptureTips(
	ctx context.Context,
	links []scriptureLink,
) []scriptureTip {
	tips := make([]scriptureTip, 0, len(links))

	for _, link := range links {
		tip, err := h.fetchScriptureTip(
			ctx,
			link.ID,
			link.Path,
		)
		if err != nil {
			fmt.Printf(
				"dailytext: tooltip failed id=%s path=%s error=%v\n",
				link.ID,
				link.Path,
				err,
			)
			continue
		}

		tips = append(tips, *tip)
	}

	return tips
}

func (h *handler) fetchScriptureTip(
	ctx context.Context,
	id string,
	scripturePath string,
) (*scriptureTip, error) {
	requestURL := wolOrigin + "/" +
		strings.TrimPrefix(scripturePath, "/")

	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		requestURL,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("create WOL request: %w", err)
	}

	request.Header.Set(
		"Accept",
		"text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
	)
	request.Header.Set(
		"User-Agent",
		"Mozilla/5.0 (compatible; DailyTextBot/1.0)",
	)

	client := &http.Client{
		Timeout: 15 * time.Second,
	}

	resp, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("fetch WOL scripture: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(
			"WOL scripture returned status %d",
			resp.StatusCode,
		)
	}

	doc, err := goquery.NewDocumentFromReader(
		io.LimitReader(resp.Body, 2<<20),
	)
	if err != nil {
		return nil, fmt.Errorf("parse WOL scripture HTML: %w", err)
	}

	scripture := doc.Find("article.bibleCitation").First()
	if scripture.Length() == 0 {
		return nil, fmt.Errorf(
			"could not find article.bibleCitation in WOL response",
		)
	}

	contentHTML, err := scripture.Html()
	if err != nil {
		return nil, fmt.Errorf("render scripture tooltip HTML: %w", err)
	}

	if contentHTML == "" {
		return nil, fmt.Errorf("WOL scripture content was empty")
	}

	title := strings.TrimSpace(
		doc.Find("meta[property='og:title']").AttrOr("content", ""),
	)

	if title == "" {
		title = strings.TrimSpace(
			doc.Find("title").First().Text(),
		)
	}

	return &scriptureTip{
		ID:        id,
		Citation:  title,
		Text:      normalizeTooltipHTML(contentHTML),
		Reference: true,
	}, nil
}

func normalizeTooltipHTML(value string) string {
	value = strings.ReplaceAll(
		value,
		`src="/`,
		`src="`+wolOrigin+`/`,
	)

	value = strings.ReplaceAll(
		value,
		`href="/`,
		`href="`+wolOrigin+`/`,
	)

	return value
}