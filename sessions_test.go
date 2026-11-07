package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/lacksonkapapahue/argus-desktop/internal/llm"
)

// A chat's turn keeps going when the user opens another chat: Stop there leaves it
// alone, reopening it shows its live history, and it saves into its own chat.
func TestChatKeepsRunningInTheBackground(t *testing.T) {
	t.Setenv("ARGUS_HOME", t.TempDir())
	a := NewApp()
	for _, id := range []string{"aaa", "bbb"} {
		b, _ := json.Marshal(Conv{ID: id, Title: id, Messages: []llm.Message{{Role: "user", Content: "hi " + id}}})
		_ = os.WriteFile(filepath.Join(convDir(), id+".json"), b, 0o600)
	}
	a.OpenConversation("aaa")
	run := a.screen()
	stopped := false
	a.mu.Lock()
	a.startSession(run, func() { stopped = true })
	run.hist = append(run.hist, llm.Message{Role: "assistant", Content: "working on it"})
	a.mu.Unlock()

	// The user opens another chat and presses Stop there.
	b := a.OpenConversation("bbb")
	if b.Running || a.convID != "bbb" {
		t.Fatalf("other chat: running=%v conv=%q", b.Running, a.convID)
	}
	a.Stop()
	if stopped {
		t.Fatal("Stop in another chat stopped the background turn")
	}
	if l := a.Live(); len(l) != 1 || l[0] != "aaa" {
		t.Fatalf("live = %v", l)
	}

	// The background turn saves into its own chat, not the one on screen.
	a.mu.Lock()
	run.hist = append(run.hist, llm.Message{Role: "assistant", Content: "done in aaa"})
	a.mu.Unlock()
	a.saveConv(run)
	var saved Conv
	raw, _ := os.ReadFile(filepath.Join(convDir(), "aaa.json"))
	_ = json.Unmarshal(raw, &saved)
	if n := len(saved.Messages); n != 3 || saved.Messages[2].Content != "done in aaa" {
		t.Fatalf("aaa saved %d messages: %+v", n, saved.Messages)
	}
	raw, _ = os.ReadFile(filepath.Join(convDir(), "bbb.json"))
	_ = json.Unmarshal(raw, &saved)
	if len(saved.Messages) != 1 {
		t.Fatalf("bbb was written to: %+v", saved.Messages)
	}

	// Going back shows it live, and Stop now stops it.
	back := a.OpenConversation("aaa")
	if len(back.Messages) != 3 || a.cur != run {
		t.Fatalf("reopened: %d messages, same session=%v", len(back.Messages), a.cur == run)
	}
	a.Stop()
	if !stopped {
		t.Fatal("Stop on the chat itself didn't stop it")
	}
	a.endSession(run)
	if len(a.Live()) != 0 || a.busy {
		t.Fatalf("still live after the end: %v busy=%v", a.Live(), a.busy)
	}
}

// A new chat started mid-turn gets its own session; the running one keeps its id.
func TestNewChatMidTurn(t *testing.T) {
	t.Setenv("ARGUS_HOME", t.TempDir())
	a := NewApp()
	a.conn = nil
	run := a.screen()
	a.mu.Lock()
	a.startSession(run, func() {})
	a.mu.Unlock()
	id := run.id
	if id == "" || a.convID != id {
		t.Fatalf("turn got no id: %q / %q", id, a.convID)
	}
	a.mu.Lock()
	a.convID, a.cur = "", &session{}
	a.mu.Unlock()
	if a.screen() == run || run.id != id {
		t.Fatal("new chat reused the running session")
	}
}
