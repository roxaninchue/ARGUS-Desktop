package longchat

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lacksonkapapahue/argus-desktop/internal/llm"
)

// A chat of n user/assistant pairs (with a tool call every 5th turn), about 1.2k
// characters each, and a fact seeded at turn 5.
func longChat(n int) []llm.Message {
	var h []llm.Message
	pad := strings.Repeat("words about the homeglow theme and its sections ", 25)
	for i := 0; i < n; i++ {
		u := fmt.Sprintf("message %d: %s", i, pad)
		if i == 2 {
			u = "Remember: the staging password hint is ZEBRA-42 and the deploy branch is release/7. " + pad
		}
		h = append(h, llm.Message{Role: "user", Content: u})
		if i%5 == 0 {
			h = append(h, llm.Message{Role: "assistant", ToolCalls: []llm.ToolCall{{ID: fmt.Sprint("c", i)}}})
			h[len(h)-1].ToolCalls[0].Function.Name = "file_read"
			h = append(h, llm.Message{Role: "tool", ToolCallID: fmt.Sprint("c", i), Content: "ok " + pad})
		}
		h = append(h, llm.Message{Role: "assistant", Content: fmt.Sprintf("reply %d: %s", i, pad)})
	}
	return h
}

// The fake summariser keeps any line with a fact marker and the previous note.
func fakeSum(calls *int) Summarise {
	return func(prev string, chunk []llm.Message) (string, error) {
		*calls++
		facts := []string{}
		if prev != "" {
			facts = append(facts, prev)
		}
		for _, m := range chunk {
			if strings.Contains(m.Content, "ZEBRA-42") {
				facts = append(facts, "Facts: staging password hint ZEBRA-42; deploy branch release/7")
			}
		}
		return "Goal: theme work\n" + strings.Join(facts, "\n"), nil
	}
}

func TestLongChatStaysUnderBudget(t *testing.T) {
	h := longChat(300)
	if Tokens(h) < 50000 {
		t.Fatalf("test chat too small: %d", Tokens(h))
	}
	var st ChatState
	calls := 0
	budget := Budget(0)
	// Replay the chat turn by turn, as the app does.
	for end := 2; end <= len(h); end += 2 {
		out, extra, _, err := Context(h[:end], &st, budget, fakeSum(&calls))
		if err != nil {
			t.Fatal(err)
		}
		if got := Tokens(out) + len(extra)/4; got > budget+Tokens(h[tailStart(h[:end]):end])+200 {
			t.Fatalf("at %d messages the prompt is %d tokens", end, got)
		}
		if len(out) > 0 && out[0].Role != "user" {
			t.Fatalf("context starts on %q", out[0].Role)
		}
	}
	out, extra, _, _ := Context(h, &st, budget, fakeSum(&calls))
	if Tokens(out)+len(extra)/4 > 6000 {
		t.Fatalf("final prompt %d tokens, want under ~6k", Tokens(out)+len(extra)/4)
	}
	t.Logf("summariser calls %d, final prompt %d tokens, chat %d tokens", calls, Tokens(out)+len(extra)/4, Tokens(h))
	if calls == 0 || calls > 60 {
		t.Fatalf("summariser ran %d times (should update incrementally)", calls)
	}
	if !strings.Contains(extra, "ZEBRA-42") || !strings.Contains(extra, "release/7") {
		t.Fatalf("summary lost the seeded fact: %q", extra)
	}
}

func TestShortChatUntouched(t *testing.T) {
	h := longChat(3)
	var st ChatState
	out, extra, changed, _ := Context(h, &st, Budget(0), fakeSum(new(int)))
	if changed || extra != "" || len(out) != len(h) {
		t.Fatal("a short chat goes word for word")
	}
}

func TestSearchChatFindsTurnFive(t *testing.T) {
	d, err := Open(filepath.Join(t.TempDir(), "lc.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	h := longChat(20)
	var ms []Msg
	for i, m := range h {
		ms = append(ms, Msg{Turn: i, Role: m.Role, Content: m.Content})
	}
	ms[5].Content = "The client's favourite colour is ultramarine, use it for buttons."
	if err := d.Index("c1", len(h), ms); err != nil {
		t.Fatal(err)
	}
	_ = d.Index("c2", 1, []Msg{{Turn: 0, Role: "user", Content: "ultramarine in another chat"}})
	hits := d.Search("c1", "what colour? ultramarine!", 5)
	if len(hits) != 1 || hits[0].Turn != 5 || !strings.Contains(hits[0].Snippet, "ultramarine") {
		t.Fatalf("hits %+v", hits)
	}
	if m, ok := d.Turn("c1", 5); !ok || !strings.Contains(m.Content, "buttons") {
		t.Fatalf("read_turn: %+v", m)
	}
	if d.Indexed("c1") != len(h) {
		t.Fatal("high-water mark")
	}
	if len(d.Search("c1", "\"); DROP", 5)) != 0 {
		t.Fatal("odd input must not break the query")
	}
	st := ChatState{Summary: "Goal: x", Upto: 12}
	_ = d.SetState("c1", st)
	if got := d.State("c1"); got != st || d.Indexed("c1") != len(h) {
		t.Fatalf("state %+v", got)
	}
	b, _ := d.SetBrief("p1", strings.Repeat("b", BriefMax+50))
	if len([]rune(b)) != BriefMax || d.Brief("p1") != b {
		t.Fatal("brief cap")
	}
	d.Forget("c1")
	if len(d.Search("c1", "ultramarine", 5)) != 0 || d.State("c1").Summary != "" {
		t.Fatal("forget")
	}
}

func TestArtifactAsideAndReadBack(t *testing.T) {
	t.Setenv("ARGUS_HOME", t.TempDir())
	if Aside("small") != "small" {
		t.Fatal("small results stay")
	}
	var lines []string
	for i := 1; i <= 500; i++ {
		lines = append(lines, fmt.Sprintf("line %03d of the build log", i))
	}
	big := strings.Join(lines, "\n")
	shown := Aside(big)
	if len(shown) >= len(big) || !strings.Contains(shown, "line 001") || !strings.Contains(shown, "line 040") || strings.Contains(shown, "line 041") || !strings.Contains(shown, "line 500") || !strings.Contains(shown, "(500 lines") {
		t.Fatalf("shown:\n%s", shown)
	}
	id := shown[strings.Index(shown, "artifact:")+9:][:12]
	got, err := ReadArtifact("artifact:"+id, 250, 3)
	if err != nil || !strings.Contains(got, "250\tline 250") || !strings.Contains(got, "252\tline 252") || strings.Contains(got, "line 253 ") {
		t.Fatalf("read back %q %v", got, err)
	}
	if _, err := ReadArtifact("../../etc/passwd", 1, 1); err == nil {
		t.Fatal("ids are checked")
	}
	one := strings.Repeat("x", 9000) // one long line
	if s := Aside(one); len(s) >= len(one) || !strings.Contains(s, "artifact:") {
		t.Fatal("long single line")
	}
}
