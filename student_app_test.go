package main

import (
	"strings"
	"testing"

	"github.com/lacksonkapapahue/argus-desktop/internal/llm"
	"github.com/lacksonkapapahue/argus-desktop/internal/tools"
)

func TestLowDataMode(t *testing.T) {
	a := selfTestApp(t)
	all := tools.All()
	has := func(ts []llm.Tool, n string) bool {
		for _, x := range ts {
			if x.Function.Name == n {
				return true
			}
		}
		return false
	}
	if !has(a.modeTools(all), "see_screen") || strings.Contains(a.system("hi"), "LOW-DATA") {
		t.Fatal("off by default")
	}
	a.prefs.LowData = true
	if has(a.modeTools(all), "see_screen") || !has(a.modeTools(all), "read_screen") {
		t.Fatal("low-data hides screenshots but keeps text screen reading")
	}
	if !strings.Contains(a.system("hi"), "LOW-DATA MODE") {
		t.Fatal("prompt line")
	}
	if n := callBytes([]llm.Message{{Role: "user", Content: strings.Repeat("x", 1000)}}, &llm.Result{Text: "ok"}); n < 1002 {
		t.Fatal("bytes", n)
	}
}

func TestFlashcardTools(t *testing.T) {
	a := selfTestApp(t)
	out := tools.Run(a.env, "flashcards_add", `{"deck":"NET103","cards":"1. Port of HTTPS :: 443\nWhat layer is IP? :: Network\nnot a card"}`)
	if !strings.Contains(out, "Saved 2") {
		t.Fatal(out)
	}
	if d := a.FlashDecks(); len(d) != 1 || d[0].Due != 2 {
		t.Fatalf("%+v", d)
	}
	cs := a.FlashDue("NET103")
	if len(cs) != 2 || cs[0].Front == "1. Port of HTTPS" {
		t.Fatalf("%+v", cs)
	}
	if e := a.FlashReview(cs[0].ID, 2); e != "" || len(a.FlashDue("NET103")) != 1 {
		t.Fatal("review", e)
	}
	if !strings.Contains(tools.Run(a.env, "flashcards_due", `{}`), "NET103 · 2 cards · 1 due") {
		t.Fatal(tools.Run(a.env, "flashcards_due", `{}`))
	}
}
