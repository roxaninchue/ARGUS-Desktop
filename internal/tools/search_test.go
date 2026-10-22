package tools

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// v0.19.13: DuckDuckGo answering with no results no longer ends the search (ported from Blaze CLI).
func TestWebSearchFallsBack(t *testing.T) {
	old := searchGet
	defer func() { searchGet = old }()
	var asked []string
	searchGet = func(_ context.Context, u string) (int, string, error) {
		asked = append(asked, u)
		switch {
		case strings.Contains(u, "news.google.com"):
			return 0, "", errors.New("dial tcp: Bad Gateway")
		case strings.Contains(u, "duckduckgo"):
			return 202, "", nil
		case strings.Contains(u, "bing.com"):
			return 200, `<?xml version="1.0"?><rss><channel><item><title>AI story</title><link>https://ex.com/a</link><description>Big &lt;b&gt;news&lt;/b&gt;</description><pubDate>Wed, 21 Oct 2026 07:22:00 GMT</pubDate></item></channel></rss>`, nil
		}
		return 500, "", nil
	}
	out := webSearch(context.Background(), "latest ai news", 5)
	if !strings.Contains(out, "AI story") || !strings.Contains(out, "https://ex.com/a") || !strings.Contains(out, "21 Oct 2026") || !strings.Contains(out, "Bing") {
		t.Fatalf("out = %s", out)
	}
	if !strings.Contains(asked[0], "news.google.com") {
		t.Errorf("news query should try Google News first: %v", asked)
	}
	asked = nil
	webSearch(context.Background(), "golang generics tutorial", 5)
	if strings.Contains(asked[0], "news.google.com") {
		t.Errorf("plain query went to news: %v", asked)
	}
}

func TestWebSearchAllDown(t *testing.T) {
	old := searchGet
	defer func() { searchGet = old }()
	searchGet = func(_ context.Context, u string) (int, string, error) { return 0, "", errors.New("Bad Gateway") }
	out := webSearch(context.Background(), "latest ai news", 5)
	if !strings.HasPrefix(out, "Error:") || !strings.Contains(out, "Don't retry search") {
		t.Fatalf("out = %s", out)
	}
}

func TestParseDDG(t *testing.T) {
	body := `<a rel="nofollow" class="result__a" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fgo.dev%2F&amp;rut=x">Go</a> ... <a class="result__snippet" href="x">The Go language</a>`
	hs := parseDDG(body, 5)
	if len(hs) != 1 || hs[0].Link != "https://go.dev/" || hs[0].Title != "Go" {
		t.Fatalf("%+v", hs)
	}
}
