package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lacksonkapapahue/argus-desktop/internal/llm"
	"github.com/lacksonkapapahue/argus-desktop/internal/tools"
)

func modeApp(t *testing.T, work, appr string) *App {
	t.Helper()
	t.Setenv("ARGUS_HOME", t.TempDir())
	a := NewApp()
	a.prefs.Composer = ComposerPrefs{WorkMode: work, Approval: appr, Migrated: true}
	a.prefs.Composer.defaults()
	return a
}

func names(ts []llm.Tool) map[string]bool {
	m := map[string]bool{}
	for _, t := range ts {
		m[t.Function.Name] = true
	}
	return m
}

// Plan mode is enforced in Go: the model never sees a tool that can change things.
func TestPlanModeFiltersTools(t *testing.T) {
	all := tools.All()
	a := modeApp(t, "plan", "auto")
	got := names(a.modeTools(all))
	for _, n := range []string{"file_write", "file_edit", "file_delete", "run_shell", "run_code", "install_tool", "job_input", "job_stop", "http_request", "download_file", "schedule", "remember", "forget", "write_skill", "open_url"} {
		if got[n] {
			t.Errorf("Plan mode offers %s", n)
		}
	}
	for _, n := range []string{"file_read", "file_list", "file_search", "web_search", "fetch_page", "system_info", "git"} {
		if !got[n] {
			t.Errorf("Plan mode is missing read/scout tool %s", n)
		}
	}
	for n := range got {
		if !planTools[n] {
			t.Errorf("Plan mode offers %s, which isn't on the read/scout list", n)
		}
	}
	if len(got) == 0 || len(got) >= len(all) {
		t.Fatalf("Plan mode kept %d of %d tools", len(got), len(all))
	}
}

func TestBuildModeKeepsEveryTool(t *testing.T) {
	all := tools.All()
	a := modeApp(t, "build", "auto")
	if n := len(a.modeTools(all)); n != len(all) {
		t.Fatalf("Build mode kept %d of %d tools", n, len(all))
	}
}

// A call the model makes anyway (a hallucinated or remembered tool) is refused in Plan.
func TestPlanModeRefusesCalls(t *testing.T) {
	a := modeApp(t, "plan", "auto")
	cases := []struct {
		name, args string
		refused    bool
	}{
		{"run_shell", `{"command":"rm -rf build"}`, true},
		{"file_write", `{"path":"a.txt","content":"x"}`, true},
		{"file_delete", `{"path":"a.txt"}`, true},
		{"file_read", `{"path":"a.txt"}`, false},
		{"web_search", `{"query":"x"}`, false},
		{"git", `{"args":"status"}`, false},
		{"git", `{"args":"log --oneline -5"}`, false},
		{"git", `{"args":"commit -am wip"}`, true},
		{"git", `{"args":"push --force"}`, true},
		{"git", `{"args":"branch -D main"}`, true},
		{"git", `{"args":"stash"}`, true},
		{"git", `{"args":"config user.name x"}`, true},
		{"git", `{"args":"config --get user.name"}`, false},
	}
	for _, c := range cases {
		why := a.modeBlock(c.name, c.args)
		if (why != "") != c.refused {
			t.Errorf("%s %s: refused=%v (%q)", c.name, c.args, why != "", why)
		}
		if why != "" && !strings.HasPrefix(why, "Refused") {
			t.Errorf("refusal should start with Refused so the UI marks the step: %q", why)
		}
	}
	b := modeApp(t, "build", "auto")
	if why := b.modeBlock("run_shell", `{"command":"ls"}`); why != "" {
		t.Errorf("Build refused run_shell: %q", why)
	}
}

func TestModeLineTellsTheModel(t *testing.T) {
	a := modeApp(t, "plan", "autopilot")
	l := a.modeLine()
	if !strings.Contains(l, "PLAN") || !strings.Contains(l, "AUTOPILOT") {
		t.Fatalf("mode line: %q", l)
	}
}

// Autopilot answers the agent's question for the away user and the run continues.
func TestAutopilotAutoReply(t *testing.T) {
	a := modeApp(t, "build", "autopilot")
	ap, steps := a.startAutopilot(40)
	if !ap.on || steps != 150 {
		t.Fatalf("autopilot run: on=%v steps=%d", ap.on, steps)
	}
	q := "I found two config files.\n\nShould I update both, or only `vite.config.ts`?"
	if !a.autoReply(a.cur, &ap, q) {
		t.Fatal("Autopilot didn't answer a question")
	}
	last := a.cur.hist[len(a.cur.hist)-1]
	if last.Role != "user" || !strings.Contains(last.Content, "The user is away") || !strings.Contains(last.Content, "pragmatically") || !strings.Contains(last.Content, "best practice") {
		t.Fatalf("auto-reply message: %+v", last)
	}
	if !strings.HasPrefix(last.Content, autoPrefix) {
		t.Fatalf("auto-reply should be marked for the chat view: %q", last.Content)
	}
	// A plain final answer is not a question: the run ends normally.
	if a.autoReply(a.cur, &ap, "Done. All 42 tests pass and the build takes 9.8s.") {
		t.Fatal("Autopilot replied to a statement")
	}
	// Bounded: at most 8 automatic replies per run.
	n := 1
	for i := 0; i < 20; i++ {
		if a.autoReply(a.cur, &ap, "Which one do you want?") {
			n++
		}
	}
	if n != 8 {
		t.Fatalf("auto replies = %d, want 8", n)
	}
}

func TestAutoModeDoesNotAutoReply(t *testing.T) {
	a := modeApp(t, "build", "auto")
	ap, steps := a.startAutopilot(40)
	if ap.on || steps != 40 {
		t.Fatalf("auto run: on=%v steps=%d", ap.on, steps)
	}
	if a.autoReply(a.cur, &ap, "Should I commit it?") {
		t.Fatal("Auto mode answered for the user")
	}
	if len(a.cur.hist) != 0 {
		t.Fatalf("history changed: %+v", a.cur.hist)
	}
}

func TestAsksUser(t *testing.T) {
	yes := []string{"Want me to commit it?", "Which option do you prefer:\n1. A\n2. B\n\nLet me know.", "Should I delete the old folder?\n", "Do you want the dark theme too"}
	no := []string{"", "Done: the tests pass.", "I asked myself why it failed? The cache. Fixed it and rebuilt; the build is green now and everything is committed to the branch."}
	for _, s := range yes {
		if !asksUser(s) {
			t.Errorf("missed a question: %q", s)
		}
	}
	for _, s := range no {
		if asksUser(s) {
			t.Errorf("not a question: %q", s)
		}
	}
}

// Auto shows a confirm card for risky actions; Autopilot approves them itself,
// except the hard blocklist, which it refuses without asking.
func TestGate(t *testing.T) {
	asked := 0
	a := modeApp(t, "build", "auto")
	a.ctx = context.Background()
	go func() {
		for i := 0; i < 50; i++ {
			a.mu.Lock()
			for id, ch := range a.pending {
				delete(a.pending, id)
				asked++
				ch <- 0
			}
			a.mu.Unlock()
			if asked > 0 {
				return
			}
			sleepMs(10)
		}
	}()
	if a.gate("Run a risky command?", "git push --force") {
		t.Fatal("Auto approved without asking")
	}
	if asked != 1 {
		t.Fatalf("Auto asked %d times", asked)
	}
	a.prefs.Composer.Approval = "autopilot"
	if !a.gate("Run a risky command?", "git push --force") {
		t.Fatal("Autopilot didn't approve a risky command")
	}
	if a.gate("Run a risky command?", "rm -rf ~/") {
		t.Fatal("Autopilot approved wiping the home folder")
	}
	home, _ := os.UserHomeDir()
	if a.gate("Delete?", home) {
		t.Fatal("Autopilot approved deleting the home folder")
	}
	if !a.gate("Delete?", filepath.Join(home, "code", "x", "build")) {
		t.Fatal("Autopilot refused an ordinary delete")
	}
}

func TestModesPersist(t *testing.T) {
	a := modeApp(t, "build", "auto")
	a.SetWorkMode("plan")
	a.SetApprovalMode("autopilot")
	b := NewApp()
	data, err := os.ReadFile(prefsFile())
	if err != nil {
		t.Fatal(err)
	}
	if err := jsonUnmarshal(data, &b.prefs); err != nil {
		t.Fatal(err)
	}
	b.migrateComposer()
	if b.workMode() != "plan" || b.approval() != "autopilot" {
		t.Fatalf("after restart: %s / %s", b.workMode(), b.approval())
	}
	// Junk values fall back to the safe defaults.
	a.SetWorkMode("yolo")
	a.SetApprovalMode("whatever")
	if a.workMode() != "build" || a.approval() != "auto" {
		t.Fatalf("junk modes: %s / %s", a.workMode(), a.approval())
	}
}

func TestFirstRunStartsSafe(t *testing.T) {
	t.Setenv("ARGUS_HOME", t.TempDir())
	a := NewApp()
	a.prefs.Composer.Approval = "autopilot" // whatever was there before the migration
	a.migrateComposer()
	if a.approval() != "auto" || a.workMode() != "build" || a.prefs.Composer.KeyWork != "ctrl+tab" || a.prefs.Composer.KeyAppr != "shift+tab" {
		t.Fatalf("first run: %+v", a.prefs.Composer)
	}
	if normKey("Alt + Tab") != "ctrl+tab" {
		t.Fatal("Alt+Tab must never be the work-mode key")
	}
}
