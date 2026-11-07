package main

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lacksonkapapahue/argus-desktop/internal/agents"
	"github.com/lacksonkapapahue/argus-desktop/internal/cloudsync"
	"github.com/lacksonkapapahue/argus-desktop/internal/llm"
)

func syncTestChats(t *testing.T) (*App, *appChats) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("ARGUS_HOME", home)
	c := Conv{ID: "main", Title: "Main", Messages: []llm.Message{{Role: "user", Content: "hello"}, {Role: "assistant", Content: "Hi."}}}
	b, _ := json.Marshal(c)
	if err := os.WriteFile(filepath.Join(convDir(), "main.json"), b, 0o600); err != nil {
		t.Fatal(err)
	}
	a := &App{live: map[string]*session{}}
	a.cur, a.convID = &session{id: "main", hist: c.Messages}, "main"
	ch := &appChats{a: a, f: &cloudsync.ConvFiles{Dir: convDir(), AliasPath: filepath.Join(home, "cloudsync", "alias.json")}, applied: map[string][]string{}}
	return a, ch
}

// A message from another device lands in the chat file and in the session on screen, so the
// app's next save keeps it.
func TestSyncChatsReloadOpenSession(t *testing.T) {
	a, ch := syncTestChats(t)
	if err := ch.PutMsg("main", cloudsync.ChatMsg{GID: "k3x9a0b1-lz8q4k2m1", Role: cloudsync.RoleUser, Text: "from the phone", At: 1761130001000}); err != nil {
		t.Fatal(err)
	}
	if ids := ch.reload(); len(ids) != 1 || ids[0] != "main" {
		t.Fatalf("reload = %v", ids)
	}
	h := a.cur.hist
	if len(h) != 3 || h[2].Role != "user" || h[2].Content != "from the phone" {
		t.Fatalf("open session not reloaded: %+v", h)
	}
	// A private chat is never written, and never listed.
	if err := ch.PutMsg("private-1", cloudsync.ChatMsg{GID: "k3x9a0b1-x", Role: cloudsync.RoleUser, Text: "secret"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(convDir(), "private-1.json")); err == nil {
		t.Fatal("private chat written")
	}
}

// A chat whose turn is running is never written; what was written before a turn started is
// handed back to the state (lostToTurns) so the next round applies it again.
func TestSyncChatsLeaveRunningTurnsAlone(t *testing.T) {
	a, ch := syncTestChats(t)
	block := make(chan struct{})
	defer close(block)
	ml := &agents.Mainline{Run: func(agents.Input) { <-block }}
	if err := ch.PutMsg("main", cloudsync.ChatMsg{GID: "k3x9a0b1-lz8q4k2m1", Role: cloudsync.RoleUser, Text: "early", At: 1}); err != nil {
		t.Fatal(err)
	}
	a.mu.Lock()
	a.cur.ml = ml
	a.mu.Unlock()
	ml.Submit(agents.Input{Kind: "user", Text: "go"})
	err := ch.PutMsg("main", cloudsync.ChatMsg{GID: "k3x9a0b1-lz8q4k2m2", Role: cloudsync.RoleUser, Text: "late", At: 2})
	if !errors.Is(err, errSyncChatBusy) {
		t.Fatalf("wrote into a running chat: %v", err)
	}
	lost := ch.lostToTurns()
	if len(lost) != 1 || lost[0] != "main:k3x9a0b1-lz8q4k2m1" {
		t.Fatalf("lostToTurns = %v", lost)
	}
	ch.reload() // a running session is not replaced under its turn
	if len(a.cur.hist) != 2 {
		t.Fatalf("running session reloaded: %+v", a.cur.hist)
	}
}

func TestSyncStatusIsHonest(t *testing.T) {
	t.Setenv("ARGUS_HOME", t.TempDir())
	a := &App{}
	if v := a.SyncState(); v.On || v.SignedIn || !strings.Contains(v.Status, "Sign in") {
		t.Fatalf("signed out: %+v", v)
	}
	if v := a.SyncEnable("long enough passphrase"); v.On || v.Error == "" {
		t.Fatalf("enable while signed out: %+v", v)
	}
	cases := map[string]error{
		"doesn't open":    cloudsync.ErrBadPassphrase,
		"new passphrase":  cloudsync.ErrWrongKey,
		"deleted from":    cloudsync.ErrNotSetUp,
		"expired":         &cloudsync.HTTPError{Status: 401, Code: "invalid_api_key"},
		"full (25 MB)":    &cloudsync.HTTPError{Status: 413, Code: "quota_exceeded"},
		"rate limit":      &cloudsync.HTTPError{Status: 429, Code: "rate_limited"},
		"had a problem":   &cloudsync.HTTPError{Status: 503, Code: "http_503"},
		"couldn't reach":  &net.OpError{Op: "dial", Err: errors.New("no route")},
		"same moment":     cloudsync.ErrBusy,
		"trying again la": context.DeadlineExceeded,
	}
	for want, err := range cases {
		if got := syncErrText(err, false); !strings.Contains(got, want) {
			t.Errorf("%v → %q, want %q", err, got, want)
		}
	}
}
