package main

import (
	"testing"

	"github.com/lacksonkapapahue/argus-desktop/internal/llm"
)

func TestPrivateIDs(t *testing.T) {
	if !isPrivate(privatePrefix+"abc") || isPrivate("main") || isPrivate("20261020-1200-ab12") {
		t.Fatal("isPrivate mismatch")
	}
}

func TestPrivateToolsDropWriters(t *testing.T) {
	var all []llm.Tool
	for _, n := range []string{"run_shell", "remember", "recall", "memory_save", "memory_update", "write_skill", "update_brief", "schedule", "web_search", "forget"} {
		var tl llm.Tool
		tl.Function.Name = n
		all = append(all, tl)
	}
	got := map[string]bool{}
	for _, tl := range privateTools(all) {
		got[tl.Function.Name] = true
	}
	for n := range privateBlocked {
		if got[n] {
			t.Fatalf("%s offered in a private chat", n)
		}
	}
	for _, n := range []string{"run_shell", "recall", "web_search"} {
		if !got[n] {
			t.Fatalf("%s missing from a private chat", n)
		}
	}
}
