package dailytext

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const wolOrigin = "https://wol.jw.org"

var scriptureTokenPattern = regexp.MustCompile(
	`\[([^\]]*)\]\(([^)]+)\)`,
)

var wolPathPattern = regexp.MustCompile(
	`(?i)/wol/(?:bc|dx|b|publication)/[^"\s)]+`,
)

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

	request.Header.Set("Accept", "application/json")
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

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, fmt.Errorf("read WOL scripture response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(
			"WOL scripture returned status %d: %s",
			resp.StatusCode,
			string(body),
		)
	}

	var payload scriptureAPIResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("decode WOL JSON response: %w", err)
	}

	if len(payload.Items) == 0 {
		return nil, fmt.Errorf("WOL returned no scripture items")
	}

	item := payload.Items[0]
	tooltipHTML := scriptureContentToHTML(item.Content)

	if tooltipHTML == "" {
		return nil, fmt.Errorf("WOL scripture content was empty")
	}

	return &scriptureTip{
		ID:        id,
		Citation:  item.Title,
		Text:      tooltipHTML,
		Reference: item.Did != nil,
	}, nil
}

func scriptureContentToHTML(content string) string {
	content = strings.TrimSpace(content)

	content = scriptureTokenPattern.ReplaceAllStringFunc(
		content,
		func(match string) string {
			parts := scriptureTokenPattern.FindStringSubmatch(match)
			if len(parts) != 3 {
				return html.EscapeString(match)
			}

			label := html.EscapeString(parts[1])
			href := wolURLFromToken(parts[2])

			if href == "" {
				return label
			}

			return `<a href="` + html.EscapeString(href) +
				`" target="_blank" rel="noreferrer">` +
				label +
				`</a>`
		},
	)

	content = html.EscapeString(content)

	// Restore only the <a> tags generated above after escaping normal text.
	content = strings.ReplaceAll(content, "&lt;a ", "<a ")
	content = strings.ReplaceAll(content, "&lt;/a&gt;", "</a>")
	content = strings.ReplaceAll(content, "&quot;", `"`)

	content = strings.ReplaceAll(content, "\r\n", "<br />")
	content = strings.ReplaceAll(content, "\n", "<br />")

	return content
}

func wolURLFromToken(value string) string {
	decoded, err := url.PathUnescape(strings.TrimSpace(value))
	if err == nil {
		value = decoded
	}

	path := wolPathPattern.FindString(value)
	if path == "" {
		return ""
	}

	path = strings.TrimSuffix(path, `/"`)
	path = strings.TrimSuffix(path, `/%22`)
	path = strings.Trim(path, `"`)

	return wolOrigin + path
}