package main

import (
	"strings"
	"testing"

	"github.com/lacksonkapapahue/blaze-desktop/internal/tools"
)

// v0.19.13 regression (strict e2e 2026-10-21, item "latest AI news"): run 1 called
// get_weather(London) for a news question, and in the one continuous chat later runs copied an
// earlier answer with no search. The tool texts and the per-turn line must route news to
// web_search, keep get_weather to weather, and say to search again every time.
func TestNewsRoutesToWebSearchNotWeather(t *testing.T) {
	wd := strings.ToLower(weatherTool().Function.Description)
	if !strings.Contains(wd, "only") || !strings.Contains(wd, "not for news") {
		t.Errorf("get_weather must say it is only for weather and not for news: %q", wd)
	}
	var sd string
	for _, tl := range tools.All() {
		if tl.Function.Name == "web_search" {
			sd = strings.ToLower(tl.Function.Description)
		}
	}
	if sd == "" {
		t.Fatal("web_search not registered")
	}
	for _, w := range []string{"news", "latest", "again", "not for weather"} {
		if !strings.Contains(sd, w) {
			t.Errorf("web_search description should mention %q: %q", w, sd)
		}
	}
	line := strings.ToLower(weatherLine)
	for _, w := range []string{"only a weather question", "news", "web_search", "every time", "never repeat"} {
		if !strings.Contains(line, w) {
			t.Errorf("per-turn routing line should contain %q: %q", w, line)
		}
	}
}
