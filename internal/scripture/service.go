package scripture

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"crypto/sha256"
	"encoding/hex"
	"github.com/PuerkitoBio/goquery"
	"github.com/redis/go-redis/v9"
)

const wolBaseURL = "https://wol.jw.org"

type Service struct {
	client *http.Client
	redis  *redis.Client
}

func NewService(redisClient *redis.Client) *Service {
	return &Service{
		client: &http.Client{
			Timeout: 25 * time.Second,
		},
		redis: redisClient,
	}
}

// Get is used internally by meetings and externally by /goapi/scripture.
func (s *Service) Get(ctx context.Context, rawPath string) (Response, error) {
	cleanPath, err := NormalizePath(rawPath)
	if err != nil {
		return Response{}, err
	}

cacheKey := scriptureCacheKey(cleanPath)

if cached, found, err := s.getCache(ctx, cacheKey); err == nil && found {
	return cached, nil
}

	requestURL := wolBaseURL + "/" + cleanPath

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		requestURL,
		nil,
	)
	if err != nil {
		return Response{}, fmt.Errorf("create WOL request: %w", err)
	}

	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("User-Agent", "homeapi/1.0")

	res, err := s.client.Do(req)
	if err != nil {
		return Response{}, fmt.Errorf("request WOL: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		return Response{}, fmt.Errorf(
			"WOL returned %s for %s",
			res.Status,
			cleanPath,
		)
	}

	body, err := io.ReadAll(io.LimitReader(res.Body, 10<<20))
	if err != nil {
		return Response{}, fmt.Errorf("read WOL response: %w", err)
	}

	result, err := parseWOLResponse(body)
	if err != nil {
		return Response{}, err
	}

	result.CachedAt = time.Now().UTC()

if err := s.setCache(ctx, cacheKey, result); err != nil {
	// Redis failing should not prevent a valid WOL response.
	// Log err here if you have a logger.
}
	return result, nil
}

// NormalizePath accepts:
//
// /en/wol/d/r1/lp-e/202026244
// en/wol/d/r1/lp-e/202026244
// https://wol.jw.org/en/wol/d/r1/lp-e/202026244
//
// It always returns a relative, locale-preserving WOL path:
//
// en/wol/d/r1/lp-e/202026244
func NormalizePath(raw string) (string, error) {
	raw = html.UnescapeString(strings.TrimSpace(raw))

	if raw == "" {
		return "", errors.New("scripture path is required")
	}

	if strings.HasPrefix(raw, "#") {
		return "", errors.New("fragment-only paths are not allowed")
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return "", errors.New("invalid scripture path")
	}

	if parsed.IsAbs() {
		if !strings.EqualFold(parsed.Host, "wol.jw.org") {
			return "", errors.New("only wol.jw.org URLs are allowed")
		}
	} else if parsed.Host != "" || parsed.Scheme != "" {
		return "", errors.New("absolute URLs are not allowed")
	}

	cleanPath := strings.TrimPrefix(parsed.EscapedPath(), "/")

	if cleanPath == "" ||
		strings.HasPrefix(cleanPath, "../") ||
		strings.Contains(cleanPath, "/../") {
		return "", errors.New("invalid scripture path")
	}

	if parsed.RawQuery != "" {
		cleanPath += "?" + parsed.RawQuery
	}

	return cleanPath, nil
}

func parseWOLResponse(body []byte) (Response, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	if err != nil {
		return Response{}, fmt.Errorf("parse WOL HTML: %w", err)
	}

	title := strings.TrimSpace(doc.Find("title").First().Text())

	content := doc.Find("#article").First()
	if content.Length() == 0 {
		content = doc.Find("main").First()
	}
	if content.Length() == 0 {
		content = doc.Find("body").First()
	}
	if content.Length() == 0 {
		return Response{}, errors.New("WOL content container not found")
	}

	// Do not return executable page assets to your React parser.
	content.Find("script, style, noscript").Remove()

	contentHTML, err := content.Html()
	if err != nil {
		return Response{}, fmt.Errorf("render WOL content: %w", err)
	}

	return Response{
		Content: contentHTML,
		Items: []Item{
			{
				Title:   title,
				Content: contentHTML,
			},
		},
	}, nil
}

const scriptureCacheTTL = 7 * 24 * time.Hour

func scriptureCacheKey(cleanPath string) string {
	 hash := sha256.Sum256([]byte(cleanPath))
	 return "homeapi:scripture:" + hex.EncodeToString(hash[:])
	// A URL-safe deterministic key. The Redis key is intentionally based on
	// the normalized WOL path, so equivalent URLs share a cached response.
	//return "homeapi:scripture:" + url.QueryEscape(cleanPath)
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
		scriptureCacheTTL,
	).Err()
}