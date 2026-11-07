package main

import (
	"strings"
	"testing"

	"github.com/lacksonkapapahue/argus-desktop/internal/llm"
)

func TestCleanTitle(t *testing.T) {
	cases := map[string]string{
		`"Fixing Float Rounding In Prices"`:            "Fixing Float Rounding In Prices",
		"Title: Postgres Setup For HomeGlow 🚀":         "Postgres Setup For HomeGlow",
		"**Vite Build Speed**.\nExplanation: because…": "Vite Build Speed",
		"<think>hmm</think>\nLaptop Fan Noise":         "Laptop Fan Noise",
		"One Two Three Four Five Six Seven Eight":      "One Two Three Four Five Six",
		"New chat": "",
		"   ":      "",
	}
	for in, want := range cases {
		if got := cleanTitle(in); got != want {
			t.Errorf("cleanTitle(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNeedsTitle(t *testing.T) {
	u := func(s string) llm.Message { return llm.Message{Role: "user", Content: s} }
	b := func(s string) llm.Message { return llm.Message{Role: "assistant", Content: s} }
	one := []llm.Message{u("hi"), b("hello")}
	if !needsTitle(titleMeta{}, one) {
		t.Fatal("first full exchange should title")
	}
	if needsTitle(titleMeta{}, []llm.Message{u("hi")}) {
		t.Fatal("no title before the reply")
	}
	if needsTitle(titleMeta{Title: "X", Source: titleManual}, one) {
		t.Fatal("manual titles are never replaced")
	}
	h := one
	for i := 0; i < 3; i++ {
		h = append(h, u("more"), b("ok"))
	}
	if needsTitle(titleMeta{Title: "X", Source: titleAuto, Turns: 1}, h[:6]) {
		t.Fatal("too soon to retitle")
	}
	if !needsTitle(titleMeta{Title: "X", Source: titleAuto, Turns: 1}, append(h, u("again"), b("ok"))) {
		t.Fatal("retitle every 4 user turns")
	}
	long := append(append([]llm.Message{}, one...), u("a"), b("b"), u(strings.Repeat("new topic ", 80)), b("ok"))
	if !needsTitle(titleMeta{Title: "X", Source: titleAuto, Turns: 1}, long) {
		t.Fatal("a long new message after 2 turns retitles")
	}
}

func TestTitlePromptIsSmall(t *testing.T) {
	var h []llm.Message
	for i := 0; i < 40; i++ {
		h = append(h, llm.Message{Role: "user", Content: strings.Repeat("x", 2000)}, llm.Message{Role: "tool", Content: strings.Repeat("y", 9000)}, llm.Message{Role: "assistant", Content: strings.Repeat("z", 2000)})
	}
	m := titlePrompt(h, "Old")
	if n := len(m[1].Content); n > 4000 {
		t.Fatalf("title prompt too big: %d chars", n)
	}
	if strings.Contains(m[1].Content, "yyy") {
		t.Fatal("tool output leaked into the title prompt")
	}
}

func TestPersonality(t *testing.T) {
	a := &App{}
	if p := a.personality(); !strings.Contains(p, "PERSONALITY\nYou are Argus. Be warm, sharp") || !strings.Contains(p, "not a person.") {
		t.Fatalf("default personality missing: %q", p)
	}
	a.prefs.Personality = "plain"
	if a.personality() != "" {
		t.Fatal("plain should have no personality block")
	}
	a.prefs.Personality, a.prefs.PersonalityCustom = "custom", "Be a pirate."
	if p := a.personality(); p != "\nPERSONALITY\nBe a pirate.\n" {
		t.Fatalf("custom: %q", p)
	}
}
