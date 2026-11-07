package tools

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/lacksonkapapahue/argus-desktop/internal/llm"
	"github.com/lacksonkapapahue/argus-desktop/internal/store"
)

func names(ts []llm.Tool) map[string]bool {
	m := map[string]bool{}
	for _, t := range ts {
		m[t.Function.Name] = true
	}
	return m
}

func TestFindToolsActivates(t *testing.T) {
	testHome(t, t.TempDir())
	SyncIndex()
	conv := "chat:test"
	e := &Env{Conv: func() string { return conv }}
	before := names(Select(conv))
	if !before["run_shell"] || !before["find_tools"] || !before["recall"] || before["download_file"] || before["use_skill"] {
		t.Fatalf("core set wrong: %v", before)
	}
	r := Run(e, "find_tools", `{"query":"download a file from a url"}`)
	if !strings.Contains(r, "download_file") {
		t.Fatalf("find_tools: %s", r)
	}
	if !names(Select(conv))["download_file"] {
		t.Error("found tools stay on for the conversation")
	}
	if names(Select("chat:other"))["download_file"] {
		t.Error("another conversation keeps its own set")
	}
	// Calling an extra tool by its exact name turns it on too.
	Run(e, "system_info", `{}`)
	if !names(Select(conv))["system_info"] {
		t.Error("a direct call should activate the tool")
	}
	if !strings.Contains(Run(e, "find_tools", `{"query":""}`), "schedule") {
		t.Error("an empty query lists the extra tools")
	}
	if strings.Contains(ExtraLine(conv), "download_file") || !strings.Contains(ExtraLine(conv), "schedule") {
		t.Errorf("ExtraLine lists only what's still off: %s", ExtraLine(conv))
	}
}

func schemaTokens(ts []llm.Tool) int {
	b, _ := json.Marshal(ts)
	return store.Tokens(string(b))
}

// TestTokenSavings measures the prompt before (whole memory.md + every tool schema)
// and after (pinned + matching memories, core tools + a names line) for 200 facts
// and 60 tools. Run with -v to see the numbers.
func TestTokenSavings(t *testing.T) {
	testHome(t, t.TempDir())
	d := DB()
	subjects := []string{"HomeGlow", "Argus Desktop", "the Android app", "the Shopify store", "Payoneer", "PayPal", "Coursera", "the CLI", "Kaggle", "TikTok ads"}
	verbs := []string{"uses", "prefers", "deploys", "tests", "builds", "keeps notes on", "pays for", "checks", "reviews", "plans"}
	things := []string{"pnpm instead of npm", "dark mode everywhere", "short Markdown answers", "a weekly sales report on Monday", "Go 1.26 with Wails v2", "vitest for unit tests", "prices rounded to cents", "Lusaka time (CAT)", "screenshots in CI", "a release every Friday"}
	var md strings.Builder
	n := 0
	for i := 0; n < 200; i++ {
		f := fmt.Sprintf("For %s Luke %s %s (case %d)", subjects[i%10], verbs[(i/10)%10], things[(i*7)%10], i)
		if _, st, _ := d.Add(f, "", "", time.Time{}); st == store.Added {
			md.WriteString("- " + f + " (2026-10-09)\n")
			n++
		}
	}
	// 60 tools: the real registry plus look-alike extras with real-sized schemas.
	all := All()
	real := len(all)
	extras := Extra()
	for i := 0; len(all) < 60; i++ {
		src := registry[extras[i%len(extras)]].tool
		cp := src
		cp.Function.Name = fmt.Sprintf("%s_%d", src.Function.Name, i)
		all = append(all, cp)
	}
	var coreTools []llm.Tool
	var offNames []string
	for _, t := range all {
		if core[t.Function.Name] {
			coreTools = append(coreTools, t)
		} else {
			offNames = append(offNames, t.Function.Name)
		}
	}
	extraLine := "\nMORE TOOLS (not loaded; find_tools(query) turns them on): " + strings.Join(offNames, ", ") + "\n"

	query := "deploy HomeGlow to the Shopify store and run the vitest tests"
	beforeMem := store.Tokens("\nWHAT YOU KNOW ABOUT THE USER\n" + md.String())
	afterMem := store.Tokens(MemoryFor(query))
	beforeTools := schemaTokens(all)
	afterTools := schemaTokens(coreTools) + store.Tokens(extraLine)
	t.Logf("memory (200 facts): before %d tokens, after %d tokens", beforeMem, afterMem)
	t.Logf("tools (60, %d real): before %d tokens, after %d tokens (%d core schemas + names line)", real, beforeTools, afterTools, len(coreTools))
	t.Logf("total per turn: before %d, after %d (%.0f%% fewer)", beforeMem+beforeTools, afterMem+afterTools, 100*(1-float64(afterMem+afterTools)/float64(beforeMem+beforeTools)))
	if afterMem > 700 || afterMem*4 > beforeMem {
		t.Errorf("memory block too big: %d vs %d", afterMem, beforeMem)
	}
	if afterTools*2 > beforeTools {
		t.Errorf("tool list too big: %d vs %d", afterTools, beforeTools)
	}
}

// A message naming a CSV turns the CSV tools on before the turn (a Cloud run's tools are fixed at start).
func TestActivateForCSV(t *testing.T) {
	testHome(t, t.TempDir())
	for _, n := range []string{"csv_report", "csv_summary"} {
		if _, ok := registry[n]; !ok {
			Register(n, "test "+n, nil, nil, func(e *Env, a map[string]any) string { return "ok" })
		}
	}
	SyncIndex()
	if got := ActivateFor("chat:a", "What's the capital of Zambia?"); len(got) != 0 || names(Select("chat:a"))["csv_report"] {
		t.Fatalf("turned on without a CSV: %v", got)
	}
	ActivateFor("chat:b", "Make a CSV report of sales-4.csv in this folder")
	if s := names(Select("chat:b")); !s["csv_report"] || !s["csv_summary"] {
		t.Fatalf("csv tools not on: %v", s)
	}
}
