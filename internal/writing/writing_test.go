package writing

import (
	"strings"
	"testing"
)

func TestSplitJoinsBack(t *testing.T) {
	text := strings.Repeat("A sentence here. Another one.\n", 200) + "\n\n" + strings.Repeat("word ", 1500)
	parts := Split(text, MaxChunk)
	if len(parts) < 3 || strings.Join(parts, "") != text {
		t.Fatalf("split lost text: %d parts", len(parts))
	}
	for _, p := range parts {
		if len(p) > MaxChunk {
			t.Fatalf("chunk too big: %d", len(p))
		}
	}
}

func TestClean(t *testing.T) {
	in := "  hello there, {name}\n"
	cases := map[string]string{
		"Here's the rewritten text:\n\nHi there, {name}":   "  Hi there, {name}\n",
		"\"Hi there, {name}\"":                             "  Hi there, {name}\n",
		"```\nHi there, {name}\n```":                       "  Hi there, {name}\n",
		"Sure! Here is a friendlier version:\nHi, {name}!": "  Hi, {name}!\n",
		"Hi there, {name}":                                 "  Hi there, {name}\n",
	}
	for out, want := range cases {
		if got := Clean(out, in); got != want {
			t.Errorf("Clean(%q) = %q, want %q", out, got, want)
		}
	}
	// A quoted input keeps its quotes.
	if got := Clean(`"Yes."`, `"Yes"`); got != `"Yes."` {
		t.Errorf("quoted input lost its quotes: %q", got)
	}
}

func TestCheck(t *testing.T) {
	in := "Email ops@blaze.test or see https://blazeinchue.pages.dev/dl, then run `blaze up` with $BLAZE_TOKEN for {{user}}."
	if err := Check("rewrite", in, "Write to ops@blaze.test, see https://blazeinchue.pages.dev/dl, run `blaze up` with $BLAZE_TOKEN for {{user}}."); err != nil {
		t.Fatalf("good rewrite rejected: %v", err)
	}
	if err := Check("rewrite", in, "Email the team or check the downloads page."); err == nil || !strings.Contains(err.Error(), "https://blazeinchue.pages.dev/dl") {
		t.Fatalf("lost link not caught: %v", err)
	}
	if err := Check("translate", "Good morning, how are you today?", "Good morning, how are you today?"); err == nil {
		t.Fatal("echo not caught")
	}
	if err := Check("translate", "Bonjour", "  "); err == nil {
		t.Fatal("empty not caught")
	}
}

func TestSystem(t *testing.T) {
	if _, err := System("translate", ""); err == nil {
		t.Fatal("translate needs a language")
	}
	s, err := System("rewrite", "Friendlier")
	if err != nil || !strings.Contains(s, "Friendlier") || !strings.Contains(s, "Return ONLY") {
		t.Fatalf("%v %q", err, s)
	}
}
