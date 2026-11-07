package main

import (
	"io/fs"
	"testing"

	"github.com/lacksonkapapahue/argus-desktop/internal/llm"
	"github.com/lacksonkapapahue/argus-desktop/internal/skillpack"
)

func TestEmbeddedPackLoads(t *testing.T) {
	sub, err := fs.Sub(skillsFS, "skills")
	if err != nil {
		t.Fatal(err)
	}
	p, err := skillpack.Load(sub)
	if err != nil || len(p) < 15 {
		t.Fatalf("embedded pack: %d skills, %v", len(p), err)
	}
}

func TestTurnSlice(t *testing.T) {
	call := func(n string) llm.Message {
		m := llm.Message{Role: "assistant"}
		m.ToolCalls = make([]llm.ToolCall, 1)
		m.ToolCalls[0].Function.Name = n
		return m
	}
	h := []llm.Message{
		{Role: "user", Content: "old question"},
		{Role: "assistant", Content: "old answer"},
		{Role: "user", Content: "From now on use pnpm in HomeGlow"},
		call("run_shell"), {Role: "tool", Content: "ok"},
		call("remember"), {Role: "tool", Content: "Remembered (#3)."},
		{Role: "assistant", Content: "Done - switched to pnpm."},
	}
	tr := turnSlice(h)
	if tr.User != "From now on use pnpm in HomeGlow" || tr.Reply != "Done - switched to pnpm." || !tr.SavedMemory || tr.SavedSkill || len(tr.Tools) != 2 {
		t.Fatalf("turn: %+v", tr)
	}
	if n, ok := parseRef("#12"); !ok || n != 12 {
		t.Fatal("parseRef")
	}
	if _, ok := parseRef("x"); ok {
		t.Fatal("parseRef accepted junk")
	}
}
