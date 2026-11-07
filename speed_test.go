package main

import (
	"reflect"
	"strings"
	"testing"

	"github.com/lacksonkapapahue/argus-desktop/internal/llm"
)

func tl(names ...string) []llm.Tool {
	out := make([]llm.Tool, len(names))
	for i, n := range names {
		out[i].Function.Name = n
	}
	return out
}

func toolNames(ts []llm.Tool) []string {
	out := make([]string, len(ts))
	for i, t := range ts {
		out[i] = t.Function.Name
	}
	return out
}

// A tool that joins mid-turn (find_tools turned it on, and tools.Select lists it in the middle)
// goes after the ones already sent; a tool that leaves just drops out; the set never changes.
func TestKeepToolOrder(t *testing.T) {
	step1, order := keepToolOrder(nil, tl("file_read", "run_shell", "web_search"))
	if !reflect.DeepEqual(toolNames(step1), []string{"file_read", "run_shell", "web_search"}) {
		t.Fatal(toolNames(step1))
	}
	step2, order := keepToolOrder(order, tl("file_read", "git_diff", "run_shell", "web_search"))
	if !reflect.DeepEqual(toolNames(step2), []string{"file_read", "run_shell", "web_search", "git_diff"}) {
		t.Fatal(toolNames(step2))
	}
	step3, _ := keepToolOrder(order, tl("git_diff", "file_read", "web_search")) // run_shell left (Plan mode)
	if !reflect.DeepEqual(toolNames(step3), []string{"file_read", "web_search", "git_diff"}) {
		t.Fatal(toolNames(step3))
	}
	dup, _ := keepToolOrder([]string{"a"}, tl("b", "a", "a"))
	if len(dup) != 3 {
		t.Fatal("the set of tools must not change", toolNames(dup))
	}
}

// The old order put the live lines (current folder, memory, clock) in the middle and the big
// constant blocks after the per-step MORE TOOLS line; the new order keeps the stable and constant
// parts at the front, so two steps of a turn share a much longer start. No text is lost.
func TestPromptOrderSharesLongerStart(t *testing.T) {
	stable := strings.Repeat("RULES and PC PROFILE. ", 400)
	constant := strings.Repeat("DELEGATION and ORCHESTRATION. ", 300)
	live1 := "\nCurrent folder: /home/u/p\nMEMORY v1\nNow: Tue 20 Oct 2026 11:07 CAT\n"
	live2 := "\nCurrent folder: /home/u/p/src\nMEMORY v2\nNow: Tue 20 Oct 2026 11:08 CAT\n"
	more1, more2 := "\nMORE TOOLS: a, b, c\n", "\nMORE TOOLS: a, c\n"

	oldOrder := func(live, more string) string { return stable + live + more + constant }
	o1, o2 := oldOrder(live1, more1), oldOrder(live2, more2)
	n1, n2 := promptOrder(stable, constant, live1, more1, "", ""), promptOrder(stable, constant, live2, more2, "", "")

	if len(n1) != len(o1) {
		t.Fatalf("reordering must keep every character: %d vs %d", len(n1), len(o1))
	}
	oldShared, newShared := commonPrefix(o1, o2), commonPrefix(n1, n2)
	if newShared < len(stable)+len(constant) || newShared <= oldShared {
		t.Fatalf("shared start: old %d, new %d (stable+constant %d)", oldShared, newShared, len(stable)+len(constant))
	}
	t.Logf("shared start between two steps: old order %d chars, new order %d chars", oldShared, newShared)

	withNote := promptOrder("S", "C", "L", "M", "X", "it was cut off")
	if withNote != "SCLMX\nABOUT YOUR LAST REPLY\nit was cut off\n" {
		t.Fatalf("%q", withNote)
	}
}
