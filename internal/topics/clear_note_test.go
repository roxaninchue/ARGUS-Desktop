package topics

import (
	"strings"
	"testing"

	"github.com/lacksonkapapahue/argus-desktop/internal/llm"
)

// v0.19.13 regression: after /clear (Opts.From), the active topic's state note that only covers
// turns before the fresh window must not reach the model, and none of those turns either. Before
// the fix Text() always added the note, so an old answer could leak into the "fresh" context
// (strict e2e 2026-10-21: repeated "latest AI news" replies copied the earlier answer).
func TestClearDropsPreClearNoteAndTurns(t *testing.T) {
	h := []llm.Message{
		u("latest AI news"),
		a("OLD-HEADLINE-ANSWER from the first search"),
		u("latest AI news"),
		a("OLD-HEADLINE-ANSWER copied"),
		u("latest AI news"),
	}
	s := New()
	s.Segment(h, SegOpts{})
	act := s.Get(s.Active)
	if act == nil {
		t.Fatal("no active topic")
	}
	// The note covers messages 0-3; /clear happened before the last question.
	s.SetNote(act.ID, "Facts: OLD-HEADLINE-NOTE", 4)
	from := 4

	msgs, extra := s.Assemble(h, Opts{Budget: 12000, From: from})
	if strings.Contains(extra, "OLD-HEADLINE-NOTE") || strings.Contains(extra, "STATE NOTE") {
		t.Errorf("pre-clear note leaked into the fresh window: %q", extra)
	}
	for _, m := range msgs {
		if strings.Contains(m.Content, "OLD-HEADLINE-ANSWER") {
			t.Errorf("pre-clear turn leaked: %q", m.Content)
		}
	}
	if len(msgs) != 1 || msgs[0].Content != "latest AI news" {
		t.Errorf("only the current question should go, got %d messages", len(msgs))
	}

	// Without /clear the note is still used (unchanged behaviour).
	if _, ex := s.Assemble(h, Opts{Budget: 12000}); !strings.Contains(ex, "OLD-HEADLINE-NOTE") {
		t.Error("without /clear the topic note should be in the topic text")
	}
	// A note that reaches past the clear point still covers fresh-window turns: keep it.
	s.SetNote(act.ID, "Facts: SPANNING-NOTE", 5)
	if _, ex := s.Assemble(h, Opts{Budget: 12000, From: 4}); !strings.Contains(ex, "SPANNING-NOTE") {
		t.Error("a note covering turns after the clear point should stay")
	}
}
