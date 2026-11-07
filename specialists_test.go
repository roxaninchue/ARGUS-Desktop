package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/lacksonkapapahue/argus-desktop/internal/agents"
)

// A specialist never has more permissions than the lead: a Plan specialist, or any
// agent while the lead is in Plan, only reads; a Build specialist with its own copy
// can't write into the original folder.
func TestSpecialistPermissions(t *testing.T) {
	orig := filepath.Join(t.TempDir(), "repo")
	copyDir := filepath.Join(t.TempDir(), "wt")
	iso := &agents.Isolation{Kind: "worktree", Source: orig, Path: copyDir}
	build := &agents.Agent{Handle: "b", Kind: agents.Specialist, Mode: "build", Iso: iso}
	plan := &agents.Agent{Handle: "p", Kind: agents.Specialist, Mode: "plan"}

	a := modeApp(t, "build", "auto")
	if why := a.agentBlock(plan, "run_shell", `{"command":"ls"}`); !strings.HasPrefix(why, "Refused") {
		t.Errorf("Plan specialist ran a shell command: %q", why)
	}
	if why := a.agentBlock(plan, "file_read", `{"path":"a.txt"}`); why != "" {
		t.Errorf("Plan specialist can't read: %q", why)
	}
	if why := a.agentBlock(build, "run_shell", `{"command":"go test ./..."}`); why != "" {
		t.Errorf("Build specialist refused: %q", why)
	}
	if why := a.agentBlock(build, "file_write", `{"path":"`+filepath.ToSlash(filepath.Join(orig, "a.go"))+`"}`); !strings.Contains(why, "original folder") {
		t.Errorf("Build specialist wrote into the original: %q", why)
	}
	if why := a.agentBlock(build, "file_write", `{"path":"`+filepath.ToSlash(filepath.Join(copyDir, "a.go"))+`"}`); why != "" {
		t.Errorf("Build specialist can't write its own copy: %q", why)
	}
	if why := a.agentBlock(build, "file_edit", `{"path":"a.go"}`); why != "" {
		t.Errorf("relative paths resolve in its copy: %q", why)
	}

	// The lead switches to Plan: every agent drops to read-only with it.
	p := modeApp(t, "plan", "auto")
	if why := p.agentBlock(build, "run_shell", `{"command":"go test ./..."}`); !strings.HasPrefix(why, "Refused") {
		t.Errorf("a Build specialist kept Build while the lead is in Plan: %q", why)
	}
	bg := &agents.Agent{Handle: "bg"}
	if !p.agentPlan(bg) {
		t.Error("a background agent kept Build while the lead is in Plan")
	}
	// Plan may brief and check specialists, but applying changes waits for Build.
	for _, n := range []string{"dispatch_specialist", "check_specialist", "message_specialist", "cancel_specialist"} {
		if why := p.modeBlock(n, `{}`); why != "" {
			t.Errorf("Plan refused %s: %q", n, why)
		}
	}
	if why := p.modeBlock("apply_specialist", `{"name":"b","action":"apply"}`); why == "" {
		t.Error("Plan allowed apply_specialist")
	}
}
