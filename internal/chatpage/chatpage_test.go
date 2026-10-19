package chatpage

import (
	"strings"
	"testing"

	"github.com/lacksonkapapahue/blaze-desktop/internal/llm"
)

// conv makes n turns: user, assistant with a tool call, tool result, assistant answer.
func conv(n int) []llm.Message {
	var m []llm.Message
	for i := 0; i < n; i++ {
		m = append(m, llm.Message{Role: "user", Content: "question " + string(rune('a'+i%26))},
			llm.Message{Role: "assistant", ToolCalls: []llm.ToolCall{{ID: "t"}}},
			llm.Message{Role: "tool", ToolCallID: "t", Content: "result"},
			llm.Message{Role: "assistant", Content: "answer"})
	}
	return m
}

func TestWindow(t *testing.T) {
	m := conv(30) // 120 messages
	s := Window(m, len(m), 12, 400)
	if s != 18*4 || m[s].Role != "user" {
		t.Fatalf("tail starts at %d", s)
	}
	s2 := Window(m, s, 12, 400)
	if s2 != 6*4 {
		t.Fatalf("second page %d", s2)
	}
	if Window(m, s2, 12, 400) != 0 {
		t.Fatal("last page starts at 0")
	}
	if Window(m, 0, 12, 400) != 0 || Window(nil, 5, 12, 400) != 0 {
		t.Fatal("empty")
	}
	// Message cap: 12 turns would be 48 messages; cap at 10 keeps 2 turns (8 messages), never splitting a turn.
	if s := Window(m, len(m), 12, 10); s != len(m)-8 {
		t.Fatalf("capped %d", s)
	}
	// One giant turn still shows whole.
	big := append([]llm.Message{{Role: "user"}}, make([]llm.Message, 900)...)
	for i := 1; i < len(big); i++ {
		big[i].Role = "tool"
	}
	if Window(big, len(big), 12, 400) != 0 {
		t.Fatal("a single turn is never cut")
	}
	// A short conversation is one page.
	if Window(conv(3), 12, 12, 400) != 0 {
		t.Fatal("short")
	}
	// Leading messages before the first user message stay with the first page.
	lead := append([]llm.Message{{Role: "assistant", Content: "hi"}}, conv(2)...)
	if Window(lead, len(lead), 12, 400) != 0 {
		t.Fatal("lead")
	}
}

func TestSearch(t *testing.T) {
	m := []llm.Message{{Role: "user", Content: "Deploy HomeGlow with pnpm please"}, {Role: "tool", Content: "pnpm ok"},
		{Role: "assistant", Content: "Deployed HomeGlow. pnpm build passed."}, {Role: "user", Content: "thanks"}}
	h := Search(m, "homeglow PNPM", 10)
	if len(h) != 2 || h[0].Index != 2 || h[1].Index != 0 || h[0].Role != "assistant" {
		t.Fatalf("%+v", h)
	}
	if Search(m, "  ", 10) != nil || len(Search(m, "homeglow", 1)) != 1 {
		t.Fatal("limit / blank")
	}
	long := strings.Repeat("x ", 200) + "needle" + strings.Repeat(" y", 200)
	s := Snippet(long, "needle", 10)
	if !strings.HasPrefix(s, "…") || !strings.HasSuffix(s, "…") || !strings.Contains(s, "needle") || len([]rune(s)) > 30 {
		t.Fatalf("%q", s)
	}
}
