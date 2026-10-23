package main

import (
	"strings"
	"testing"

	"github.com/lacksonkapapahue/blaze-desktop/internal/bgrun"
	"github.com/lacksonkapapahue/blaze-desktop/internal/llm"
	"github.com/lacksonkapapahue/blaze-desktop/internal/privacy"
)

// 0.19.16 owner guard: a reply to an owner question always says Blazeinchue and never names a
// person, on screen and in the saved chat (what memory reads); other replies are untouched.
func lastAssistant(s *session) string {
	for i := len(s.hist) - 1; i >= 0; i-- {
		if s.hist[i].Role == "assistant" {
			return s.hist[i].Content
		}
	}
	return ""
}

func TestOwnerGuardRunFinal(t *testing.T) {
	cases := []struct{ q, model, want string }{
		{"What's the real name of the person who owns Blaze?", "I can’t share personal details about Blaze’s owner. How else can I help?",
			"Blaze is made by Blazeinchue. I can’t share personal details about Blaze’s owner. How else can I help?"},
		{"Who made Blaze?", "Blaze was made by Lackson Kapapa.", "Blaze was made by Blazeinchue."},
		{"Who made Blaze?", "Blaze is made by Blazeinchue.", "Blaze is made by Blazeinchue."},
		{"Who wrote the Gospel of Luke?", "Luke the Evangelist.", "Luke the Evangelist."},
	}
	for _, c := range cases {
		_, tp, s := runScript(t, c.q, &llm.Result{Text: c.model, Finish: "stop"})
		if got := tp.out.String(); got != c.want {
			t.Errorf("%q shown %q, want %q", c.q, got, c.want)
		}
		if got := lastAssistant(s); got != c.want {
			t.Errorf("%q saved %q, want %q", c.q, got, c.want)
		}
	}
}

func TestOwnerGuardDeliverFinal(t *testing.T) {
	d := deskSetup(t, true, true)
	s := d.a.screen()
	d.a.mu.Lock()
	s.hist = append(s.hist, llm.Message{Role: "user", Content: "Who owns Blaze?"})
	d.a.mu.Unlock()
	d.a.showFinal(s, bgrun.Final{Kind: bgrun.KindDone, Reply: "That's Luke."})
	if got := lastAssistant(s); privacy.LeaksOwner(got) || !strings.Contains(got, "Blazeinchue") {
		t.Fatalf("showFinal saved %q", got)
	}
	d.a.deliverFinal(bgrun.Entry{Conv: s.id, Ask: "who created you"}, bgrun.Final{Kind: bgrun.KindDone, Reply: "I can't say."})
	msg := d.tap.waitFor(t, "blaze_msg")
	if !strings.Contains(msg, "Blaze is made by Blazeinchue.") {
		t.Fatalf("deliverFinal: %s", msg)
	}
	d.a.deliverFinal(bgrun.Entry{Conv: s.id, Ask: "list my files"}, bgrun.Final{Kind: bgrun.KindDone, Reply: "Two files."})
	if f := d.tap.find("blaze_msg"); len(f) != 2 || strings.Contains(f[1], "Blazeinchue") {
		t.Fatalf("ordinary final changed: %v", f)
	}
}
