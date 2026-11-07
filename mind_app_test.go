package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lacksonkapapahue/argus-desktop/internal/llm"
	"github.com/lacksonkapapahue/argus-desktop/internal/mind"
	"github.com/lacksonkapapahue/argus-desktop/internal/tools"
)

func mindTestApp(t *testing.T) *App {
	a := selfTestApp(t)
	t.Cleanup(mind.CloseDefault)
	a.live = map[string]*session{}
	a.cur = &session{}
	a.ready = make(chan struct{})
	close(a.ready)
	return a
}

func writeChat(t *testing.T, c Conv) {
	b, _ := json.Marshal(c)
	if err := os.WriteFile(filepath.Join(convDir(), c.ID+".json"), b, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestMindMigrationIdempotent(t *testing.T) {
	a := mindTestApp(t)
	writeChat(t, Conv{ID: "20260101-old1", Title: "Exams", Updated: time.Now(), Messages: []llm.Message{{Role: "user", Content: "My finals start 16 November"}, {Role: "assistant", Content: "Good luck!"}}})
	writeChat(t, Conv{ID: "20260102-old2", Project: "p1", Title: "HomeGlow", Updated: time.Now(), Messages: []llm.Message{{Role: "user", Content: "HomeGlow sells cosy lamps"}}})
	j, _ := a.migrateMind()
	if j != 3 {
		t.Fatalf("journaled %d", j)
	}
	if j2, i2 := a.migrateMind(); j2 != 0 || i2 != 0 {
		t.Fatal("second run must do nothing", j2, i2)
	}
	d := mindDB()
	if d.Meta(migratedKey) == "" || d.CountUnreviewed() != 3 {
		t.Fatal("flag set and turns queued for review")
	}
	// Even with the flag cleared, rows are unique per (conv, turn): nothing doubles.
	_ = d.SetMeta(migratedKey, "")
	if j3, _ := a.migrateMind(); j3 != 0 {
		t.Fatal("re-import duplicated rows")
	}
	if _, err := os.Stat(filepath.Join(convDir(), "20260101-old1.json")); err != nil {
		t.Fatal("chat files are never removed")
	}
	if h := d.SearchJournal("finals november", "", 5); len(h) != 1 || h[0].Conv != "20260101-old1" {
		t.Fatalf("FTS recall over imported chats: %+v", h)
	}
	if arch := a.ArchivedConversations(); len(arch) != 1 || arch[0].ID != "20260101-old1" {
		t.Fatalf("archive: %+v", arch)
	}
	if len(a.Conversations("")) != 0 {
		t.Fatal("Chat mode lists only the main conversation")
	}
	if c := a.ReadArchived("20260101-old1"); len(c.Messages) != 2 {
		t.Fatal("archived chats stay readable")
	}
}

func TestMindIdleReviewMarksTurns(t *testing.T) {
	a := mindTestApp(t)
	d := mindDB()
	_, _ = d.Journal("20260101-old1", []mind.Msg{{Turn: 0, Role: "user", Content: "My mum Gloria pays my fees in instalments"}, {Turn: 1, Role: "assistant", Content: "Noted."}})
	old := llmQuick
	defer func() { llmQuick = old }()
	calls := 0
	llmQuick = func(ctx context.Context, a *App, msgs []llm.Message) (string, error) {
		calls++
		if !strings.Contains(msgs[1].Content, "Gloria") {
			t.Error("review should read the journal")
		}
		return `[{"kind":"person","subject":"Gloria","text":"Luke's mum; pays his fees in instalments"}]`, nil
	}
	if n := a.maybeIdleReview(context.Background(), true); n != 2 {
		t.Fatalf("marked %d", n)
	}
	if d.CountUnreviewed() != 0 || calls != 1 {
		t.Fatal("turns marked reviewed")
	}
	if r := d.Search("fees", "", 5); len(r) != 1 || r[0].Kind != "person" || r[0].Mirror == 0 {
		t.Fatalf("memory saved and mirrored to argus.db: %+v", r)
	}
	if a.maybeIdleReview(context.Background(), true) != 0 {
		t.Fatal("nothing left to review")
	}
	a.prefs.MindReview = new(bool)
	_, _ = d.Journal("c2", []mind.Msg{{Turn: 0, Role: "user", Content: "x y z words"}})
	if a.maybeIdleReview(context.Background(), true) != 0 {
		t.Fatal("the Settings toggle turns the review off")
	}
}

func TestMindToolsAndPrompt(t *testing.T) {
	a := mindTestApp(t)
	old := theApp
	theApp = a
	defer func() { theApp = old }()
	out := tools.Run(a.env, "memory_save", `{"kind":"preference","subject":"study","text":"Luke studies best after dinner on weekdays"}`)
	if !strings.HasPrefix(out, "Saved [") {
		t.Fatal(out)
	}
	out = tools.Run(a.env, "memory_save", `{"kind":"preference","subject":"study","text":"Luke studies best after dinner on weekdays, in the library"}`)
	if !strings.HasPrefix(out, "Updated [") {
		t.Fatal("same subject, newer line: update, not a duplicate:", out)
	}
	if !strings.Contains(tools.Run(a.env, "memory_search", `{"query":"library"}`), "in the library") {
		t.Fatal("search")
	}
	tools.Run(a.env, "memory_save", `{"kind":"person","subject":"Mutale","text":"In Luke's NET103 group"}`)
	sys := a.system("hi")
	for _, want := range []string{"ONE MIND", "WHAT YOU KNOW", "after dinner", "MEMORY INDEX", "person: Mutale (1)"} {
		if !strings.Contains(sys, want) {
			t.Errorf("prompt misses %q", want)
		}
	}
	if strings.Contains(sys, "NET103 group") {
		t.Error("people aren't pinned; they're looked up")
	}
	for _, n := range []string{"memory_search", "memory_save", "memory_update"} {
		if !tools.IsCore(n) {
			t.Errorf("%s should be core", n)
		}
	}
}

func TestOneThreadAndClear(t *testing.T) {
	a := mindTestApp(t)
	s := &session{}
	a.cur = s
	a.nameSession(s)
	if s.id != mainConv {
		t.Fatal("Chat mode names the one conversation main:", s.id)
	}
	a.prefs.Project = "p1"
	p := &session{}
	a.nameSession(p)
	if p.id == mainConv {
		t.Fatal("projects keep their own chats")
	}
	a.prefs.Project = ""
	s.hist = []llm.Message{{Role: "user", Content: "a"}, {Role: "assistant", Content: "b"}}
	if n := a.ClearContext(); n != 2 || s.ctxFrom != 2 {
		t.Fatal("clear sets the window start", n)
	}
	if c := a.OpenMain(); c.ContextFrom != 2 || len(c.Messages) != 2 {
		t.Fatalf("the transcript keeps everything and remembers the window: %+v", c)
	}
}

func TestLookupTail(t *testing.T) {
	h := []llm.Message{{Role: "user"}, {Role: "assistant"}, {Role: "user"}, {Role: "assistant"}, {Role: "tool"}, {Role: "assistant"}, {Role: "user"}, {Role: "assistant"}, {Role: "tool"}}
	tl := lookupTail(h)
	if tl[0].Role != "user" || len(tl) > mind.LookupTail+2 {
		t.Fatalf("%d %+v", len(tl), tl[0])
	}
}
