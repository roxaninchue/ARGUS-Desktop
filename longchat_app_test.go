package main

import (
	"context"
	"strings"
	"testing"

	"github.com/lacksonkapapahue/argus-desktop/internal/llm"
	"github.com/lacksonkapapahue/argus-desktop/internal/longchat"
)

func TestLongContextShortChatAndIndex(t *testing.T) {
	a := testApp(t)
	t.Cleanup(longchat.CloseDefault) // runs before TempDir's removal; Windows can't delete an open db
	s := &session{id: "c-long", project: "p1", started: true}
	s.hist = []llm.Message{{Role: "user", Content: "The launch date is 14 November."}, {Role: "assistant", Content: "Noted."}}
	d, err := longchat.Default()
	if err != nil {
		t.Fatal(err)
	}
	_, _ = d.SetBrief("p1", "Purpose: HomeGlow theme. Rule: use pnpm.")
	out, extra := a.longContext(context.Background(), s, s.hist)
	if len(out) != 2 || !strings.Contains(extra, "PROJECT BRIEF") || !strings.Contains(extra, "use pnpm") {
		t.Fatalf("out %d extra %q", len(out), extra)
	}
	if hits := d.Search("c-long", "launch date", 5); len(hits) != 1 || hits[0].Turn != 0 {
		t.Fatalf("indexed: %+v", hits)
	}
	// A shorter history (cleared chat) re-indexes from scratch.
	s.hist = s.hist[:1]
	a.longContext(context.Background(), s, s.hist)
	if d.Indexed("c-long") != 1 {
		t.Fatal("re-index after the chat got shorter")
	}
}

func TestAsideResult(t *testing.T) {
	t.Setenv("ARGUS_HOME", t.TempDir())
	big := strings.Repeat("a line of output\n", 1000)
	if got := asideResult("run_shell", big); len(got) >= len(big) || !strings.Contains(got, "artifact:") {
		t.Fatal("big shell output should be set aside")
	}
	if asideResult("load_skill", big) != big {
		t.Fatal("skills stay whole")
	}
}
