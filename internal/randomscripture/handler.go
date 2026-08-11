package randomscripture

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

var bibleChapters = []int{
	50, 40, 27, 36, 34, 24, 21, 4, 31, 24, 22, 25, 29, 36, 10, 13, 10,
	42, 150, 31, 12, 8, 66, 52, 5, 48, 12, 14, 3, 9, 1, 4, 7, 3, 3, 3,
	2, 14, 4, 28, 16, 24, 21, 28, 16, 16, 13, 6, 6, 4, 4, 5, 3, 6, 4,
	3, 1, 13, 5, 5, 3, 5, 1, 1, 1, 22,
}

type verse struct {
	Number  int    `json:"number"`
	Content string `json:"content"`
}

type jwBookResponse struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

type scriptureResponse struct {
	Book    int   `json:"book"`
	Chapter int   `json:"chapter"`
	Title   string `json:"title"`
	Verse   verse `json:"verse"`
}

type ScriptureHandler struct {
	Client *http.Client
	Rand   *rand.Rand
}

func NewScriptureHandler() *ScriptureHandler {
	return &ScriptureHandler{
		Client: &http.Client{
			Timeout: 15 * time.Second,
		},
		Rand: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (h *ScriptureHandler) GetRandomScripture(
	w http.ResponseWriter,
	r *http.Request,
) {
	book, chapter, verseNumber, err := h.requestValues(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	jwBook, err := h.fetchBook(r, book, chapter)
	if err != nil {
		http.Error(w, "could not retrieve scripture", http.StatusBadGateway)
		return
	}

	verses, err := parseBookHTML(jwBook.Content)
	if err != nil {
		http.Error(w, "could not parse scripture", http.StatusBadGateway)
		return
	}

	if len(verses) == 0 {
		http.Error(w, "no verses found", http.StatusNotFound)
		return
	}

	selectedVerse := verses[h.Rand.Intn(len(verses))]

	if verseNumber > 0 {
		found := false

		for _, item := range verses {
			if item.Number == verseNumber {
				selectedVerse = item
				found = true
				break
			}
		}

		if !found {
			http.Error(
				w,
				"verse not found in this chapter",
				http.StatusNotFound,
			)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	if err := json.NewEncoder(w).Encode(scriptureResponse{
		Book:    book,
		Chapter: chapter,
		Title:   jwBook.Title,
		Verse:   selectedVerse,
	}); err != nil {
		http.Error(w, "could not encode response", http.StatusInternalServerError)
	}
}

func (h *ScriptureHandler) requestValues(
	r *http.Request,
) (int, int, int, error) {
	query := r.URL.Query()

	book, _ := strconv.Atoi(query.Get("book"))
	chapter, _ := strconv.Atoi(query.Get("chapter"))
	verseNumber, _ := strconv.Atoi(query.Get("verse"))

	if book == 0 {
		book = h.Rand.Intn(66) + 1
	}

	if book < 1 || book > len(bibleChapters) {
		return 0, 0, 0, fmt.Errorf("book must be between 1 and 66")
	}

	if chapter == 0 {
		chapter = h.Rand.Intn(bibleChapters[book-1]) + 1
	}

	if chapter < 1 || chapter > bibleChapters[book-1] {
		return 0, 0, 0, fmt.Errorf(
			"book %d has chapters 1 through %d",
			book,
			bibleChapters[book-1],
		)
	}

	return book, chapter, verseNumber, nil
}

func (h *ScriptureHandler) fetchBook(
	r *http.Request,
	book int,
	chapter int,
) (jwBookResponse, error) {
	target := fmt.Sprintf(
		"https://wol.jw.org/wol/b/r1/lp-e/nwtsty/%d/%d",
		book,
		chapter,
	)

	req, err := http.NewRequestWithContext(
		r.Context(),
		http.MethodGet,
		target,
		nil,
	)
	if err != nil {
		return jwBookResponse{}, err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "homeapi/1.0")

	resp, err := h.Client.Do(req)
	if err != nil {
		return jwBookResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return jwBookResponse{}, fmt.Errorf(
			"JW upstream returned %s",
			resp.Status,
		)
	}

	var result jwBookResponse

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return jwBookResponse{}, err
	}

	return result, nil
}

// Equivalent to your original:
// const verseMatch = /^(\d+)\s(.+)/.exec(text)
var versePattern = regexp.MustCompile(
	`^(\d+)[ \t\r\n\x{00A0}\x{202F}]+(.+)$`,
)

func parseBookHTML(rawHTML string) ([]verse, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(rawHTML))
	if err != nil {
		return nil, err
	}

	verses := make([]verse, 0)

	// Equivalent to:
	// const spans = doc.querySelectorAll("p span")
	doc.Find("p span").Each(func(_ int, span *goquery.Selection) {
		// Equivalent to:
		// const text = span.textContent?.trim() ?? ""
		text := strings.TrimSpace(span.Text())

		if text == "" {
			return
		}

		// Equivalent to:
		// const verseMatch = /^(\d+)\s(.+)/.exec(text)
		match := versePattern.FindStringSubmatch(text)

		if len(match) == 3 {
			number, err := strconv.Atoi(match[1])
			if err != nil {
				return
			}

			// Equivalent to:
			// verses.push({
			//   number: Number.parseInt(verseMatch[1], 10),
			//   content: verseMatch[2],
			// })
			verses = append(verses, verse{
				Number:  number,
				Content: match[2],
			})

			return
		}

		// Equivalent to:
		// else if (verses.length > 0) {
		//   verses.at(-1)!.content += " " + text
		// }
		if len(verses) > 0 {
			last := len(verses) - 1
			verses[last].Content += " " + text
		}
	})

	return verses, nil
}