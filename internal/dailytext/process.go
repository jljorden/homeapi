package dailytext

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const wolOrigin = "https://wol.jw.org"

type scriptureLink struct {
	ID   string
	Path string
}

type scriptureAPIResponse struct {
	Items []struct {
		Title   string `json:"title"`
		Content string `json:"content"`
		Did     any    `json:"did"`
	} `json:"items"`
}

var scriptureLinkPattern = regexp.MustCompile(
	`\[([^\]]*)\]\(([^)]+)\)`,
)

var wolPathPattern = regexp.MustCompile(
	`/wol/(?:bc|dx|b|publication)/[^"\s)]+`,
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
		tip, err := h.fetchScriptureTip(ctx, link.ID, link.Path)
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
		return nil, fmt.Errorf("read WOL response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(
			"WOL returned status %d: %s",
			resp.StatusCode,
			string(body),
		)
	}

	var payload scriptureAPIResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("decode WOL JSON: %w", err)
	}

	if len(payload.Items) == 0 {
		return nil, fmt.Errorf("WOL returned no scripture items")
	}

	item := payload.Items[0]

	return &scriptureTip{
		ID:        id,
		Citation:  item.Title,
		Text:      normalizeTooltipHTML(item.Content),
		Reference: item.Did != nil,
	}, nil
}

func normalizeTooltipHTML(content string) string {
	doc, err := goquery.NewDocumentFromReader(
		strings.NewReader(
			`<div id="scripture-tooltip-root">` +
				content +
				`</div>`,
		),
	)
	if err != nil {
		return ""
	}

	root := doc.Find("#scripture-tooltip-root").First()
	if root.Length() == 0 {
		return ""
	}

	root.Find("*").Each(func(_ int, selection *goquery.Selection) {
		ownHTML, err := selection.Html()
		if err != nil || !strings.Contains(ownHTML, "](") {
			return
		}

		selection.SetHtml(convertBracketLinks(ownHTML))
	})

	result, err := root.Html()
	if err != nil {
		return ""
	}

	return result
}

func convertBracketLinks(content string) string {
	return scriptureLinkPattern.ReplaceAllStringFunc(
		content,
		func(match string) string {
			parts := scriptureLinkPattern.FindStringSubmatch(match)
			if len(parts) != 3 {
				return match
			}

			label := parts[1]
			href := normalizeTooltipLink(parts[2])

			if href == "" {
				return label
			}

			return `<a href="` + href +
				`" target="_blank" rel="noreferrer">` +
				label +
				`</a>`
		},
	)
}

func normalizeTooltipLink(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)

	decoded, err := url.PathUnescape(rawURL)
	if err == nil {
		rawURL = decoded
	}

	path := wolPathPattern.FindString(rawURL)
	if path == "" {
		return ""
	}

	path = strings.TrimSuffix(path, `/"`)
	path = strings.TrimSuffix(path, `/%22`)
	path = strings.Trim(path, `"`)

	return wolOrigin + path
}