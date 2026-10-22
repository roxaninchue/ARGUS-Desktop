package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"testing"

	"github.com/lacksonkapapahue/blaze-desktop/internal/config"
	"github.com/lacksonkapapahue/blaze-desktop/internal/llm"
)

// Script tests for App.run, the production loop, behind its two ports: completeTurn
// (the models) and eventTap (the chat view). v0.19.13, Bug Diver review holes 1, 3, 5, 6.

type scripted struct {
	mu      sync.Mutex
	replies []*llm.Result
	calls   int
	require []bool // tool_choice "required" per request
	plain   []bool // Opts.PlainFallback per request
	sys     []string
}

func (sc *scripted) complete(ctx context.Context, _ *config.Config, msgs []llm.Message, _ []llm.Tool, o llm.Opts) (*llm.Result, error) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.require = append(sc.require, llm.RequireTool(ctx))
	sc.plain = append(sc.plain, o.PlainFallback)
	if len(msgs) > 1 {
		sc.sys = append(sc.sys, msgs[1].Content)
	}
	if sc.calls >= len(sc.replies) {
		sc.calls++
		return &llm.Result{Text: "ok", Finish: "stop"}, nil
	}
	r := sc.replies[sc.calls]
	sc.calls++
	if o.OnText != nil && r.Text != "" {
		o.OnText(r.Text)
	}
	return r, nil
}

type tap struct {
	mu  sync.Mutex
	evs []string
	out strings.Builder
}

func (t *tap) on(ev string, d any) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.evs = append(t.evs, ev)
	if ev == "delta" {
		t.out.WriteString(d.(string))
	}
	if ev == "done" || ev == "error" || ev == "tool" {
		b, _ := json.Marshal(d)
		t.evs[len(t.evs)-1] += " " + string(b)
	}
}

func (t *tap) has(prefix string) string {
	for _, e := range t.evs {
		if strings.HasPrefix(e, prefix) {
			return e
		}
	}
	return ""
}

func runScript(t *testing.T, user string, replies ...*llm.Result) (*scripted, *tap, *session) {
	t.Helper()
	a := testApp(t)
	a.pv = &preview{}
	panicTap = func(r any) { t.Logf("PANIC %v\n%s", r, debug.Stack()) }
	llm.PanicTap = panicTap
	t.Cleanup(func() { panicTap, llm.PanicTap = nil, nil })
	a.prefs.Folder = t.TempDir()
	a.newEnv()
	sc := &scripted{replies: replies}
	tp := &tap{}
	oldC, oldT := completeTurn, eventTap
	completeTurn, eventTap = sc.complete, tp.on
	t.Cleanup(func() { completeTurn, eventTap = oldC, oldT })
	s := a.screen()
	s.id = privatePrefix + "script"
	s.hist = []llm.Message{{Role: "user", Content: user}}
	a.run(context.Background(), s)
	return sc, tp, s
}

// D2: an own-key model writes file_list as a <tool_call> block. It now runs, and the
// syntax is never shown.
func TestRunRecoversTextToolCall(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "marker.txt"), []byte("x"), 0o600)
	arg, _ := json.Marshal(map[string]string{"path": dir})
	call := "Let me look.\n<tool_call>{\"name\": \"file_list\", \"arguments\": " + string(arg) + "}</tool_call>"
	sc, tp, s := runScript(t, "list the files in "+dir,
		&llm.Result{Text: call, Finish: "stop"},
		&llm.Result{Text: "There is one file, marker.txt.", Finish: "stop"})
	if tp.has("tool ") == "" || !strings.Contains(tp.has("tool "), "file_list") {
		t.Fatalf("text call never ran: %v", tp.evs)
	}
	if strings.Contains(tp.out.String(), "<tool_call") {
		t.Fatalf("tool syntax shown: %q", tp.out.String())
	}
	if sc.calls != 2 || tp.has("done") == "" {
		t.Fatalf("calls %d evs %v", sc.calls, tp.evs)
	}
	var sawResult bool
	for _, m := range s.hist {
		if m.Role == "tool" && strings.Contains(m.Content, "marker.txt") {
			sawResult = true
		}
	}
	if !sawResult {
		for _, m := range s.hist {
			t.Logf("%s: %.300s", m.Role, m.Content)
		}
		t.Fatal("no tool result in history")
	}
	if len(sc.require) == 0 || !sc.require[0] {
		t.Fatal("an action request must send tool_choice required on its first step")
	}
}

// Hole 3: "Done!" with no tool is sent back once, then marked unverified.
func TestRunClaimWithoutToolIsCaught(t *testing.T) {
	sc, tp, _ := runScript(t, "set a reminder for 5pm to call mum",
		&llm.Result{Text: "Done! Your reminder is set for 5pm.", Finish: "stop"},
		&llm.Result{Text: "Done, reminder set.", Finish: "stop"})
	if sc.calls != 2 {
		t.Fatalf("claim not sent back: %d requests", sc.calls)
	}
	if !strings.Contains(sc.sys[1], "no tool ran") {
		t.Fatal("the retry carries no proof note")
	}
	d := tp.has("done")
	if !strings.Contains(d, `"unverified":true`) || !strings.Contains(tp.out.String(), "nothing was actually done") {
		t.Fatalf("unverified claim not flagged: %s / %q", d, tp.out.String())
	}
}

// A plain question is not an action: no forced tool, no claim check.
func TestRunPlainQuestionUntouched(t *testing.T) {
	sc, tp, _ := runScript(t, "what's the capital of Japan?", &llm.Result{Text: "Tokyo.", Finish: "stop"})
	if sc.calls != 1 || sc.require[0] || tp.has("done") == "" || strings.Contains(tp.has("done"), "unverified") {
		t.Fatalf("calls %d require %v evs %v", sc.calls, sc.require, tp.evs)
	}
}

// Hole 5: an empty reply is an error with Retry, not an empty done bubble.
func TestRunEmptyReplyIsError(t *testing.T) {
	_, tp, _ := runScript(t, "explain goroutines", &llm.Result{Text: "  ", Finish: "stop"})
	if tp.has("done") != "" || !strings.Contains(tp.has("error"), "empty") {
		t.Fatalf("evs %v", tp.evs)
	}
}

// Unrunnable tool syntax is retried once, then an error; never shown as the answer.
func TestRunJunkToolTextErrors(t *testing.T) {
	junk := &llm.Result{Text: "<tool_call>{\"name\": \"no_such_tool\", \"arguments\": {}}</tool_call>", Finish: "stop"}
	sc, tp, _ := runScript(t, "explain goroutines", junk, junk)
	if sc.calls != 2 || !strings.Contains(tp.has("error"), "as text") || strings.Contains(tp.out.String(), "tool_call") {
		t.Fatalf("calls %d evs %v out %q", sc.calls, tp.evs, tp.out.String())
	}
}

func TestActionIntentAndClaims(t *testing.T) {
	for _, s := range []string{"run the tests and fix the failing one", "set a reminder for 5pm", "post this on X", "create notes.txt with hi", "what's the weather in Lusaka today?", "look for latest ai news"} {
		if !actionIntent(s) {
			t.Errorf("want action: %q", s)
		}
	}
	for _, s := range []string{"hi", "thanks!", "what's the capital of Japan?", "explain how goroutines work", "how do I run tests in go?"} {
		if actionIntent(s) {
			t.Errorf("not action: %q", s)
		}
	}
	for _, s := range []string{"Done! Posted to X.", "I've created notes.txt.", "Your reminder is set for 5pm.", "All tests passed."} {
		if !claimsDone(s) {
			t.Errorf("want claim: %q", s)
		}
	}
	for _, s := range []string{"I can't post to X from here.", "Tokyo.", "Here's how goroutines work."} {
		if claimsDone(s) {
			t.Errorf("not a claim: %q", s)
		}
	}
}

func listCall(t *testing.T, dir string) *llm.Result {
	t.Helper()
	arg, _ := json.Marshal(map[string]string{"path": dir})
	tc := llm.ToolCall{ID: "c1", Type: "function"}
	tc.Function.Name = "file_list"
	tc.Function.Arguments = string(arg)
	return &llm.Result{ToolCalls: []llm.ToolCall{tc}, Finish: "tool_calls"}
}

// Hole 3: a claim backed by a tool that ran this turn is left alone.
func TestRunClaimAfterToolNotFlagged(t *testing.T) {
	dir := t.TempDir()
	sc, tp, _ := runScript(t, "list the files in "+dir,
		listCall(t, dir),
		&llm.Result{Text: "Done! I've listed the folder: it's empty.", Finish: "stop"})
	if sc.calls != 2 || strings.Contains(tp.has("done"), "unverified") || strings.Contains(tp.out.String(), "nothing was actually done") {
		t.Fatalf("calls %d evs %v out %q", sc.calls, tp.evs, tp.out.String())
	}
}

// Hole 3: the corrective retry that then calls a tool ends as a normal, verified turn.
func TestRunClaimRetryThenTool(t *testing.T) {
	dir := t.TempDir()
	sc, tp, _ := runScript(t, "list the files in "+dir,
		&llm.Result{Text: "Done! I've listed the files.", Finish: "stop"},
		listCall(t, dir),
		&llm.Result{Text: "The folder is empty.", Finish: "stop"})
	if sc.calls != 3 || !strings.Contains(sc.sys[1], "no tool ran") {
		t.Fatalf("calls %d sys %q", sc.calls, sc.sys)
	}
	if !strings.Contains(tp.has("tool "), "file_list") || tp.has("done") == "" || strings.Contains(tp.has("done"), "unverified") {
		t.Fatalf("evs %v", tp.evs)
	}
}

// Hole 3 must not fire on plain answers, even ones that read like a claim.
func TestRunClaimCheckSkipsPlainAnswers(t *testing.T) {
	for _, c := range []struct{ user, reply string }{
		{"what's 2+2?", "Done the maths: 4."},
		{"explain how git push works", "I've written a short explanation: git push has been around since 2005 and sends commits to a remote."},
		{"thanks!", "All set, anytime."},
	} {
		sc, tp, _ := runScript(t, c.user, &llm.Result{Text: c.reply, Finish: "stop"})
		if sc.calls != 1 || strings.Contains(tp.has("done"), "unverified") || strings.Contains(tp.out.String(), "nothing was actually done") {
			t.Errorf("%q: calls %d evs %v", c.user, sc.calls, tp.evs)
		}
	}
}

// Hole 4: the keyless plain fallback only for turns without action intent.
func TestRunPlainFallbackOnlyWithoutIntent(t *testing.T) {
	sc, _, _ := runScript(t, "what's the capital of Japan?", &llm.Result{Text: "Tokyo.", Finish: "stop"})
	if len(sc.plain) != 1 || !sc.plain[0] {
		t.Fatalf("plain question: PlainFallback %v, want true", sc.plain)
	}
	sc, _, _ = runScript(t, "post this on X: hello world",
		&llm.Result{Text: "I can't post to X from here: no X tool is connected.", Finish: "stop"})
	if len(sc.plain) == 0 {
		t.Fatal("no request made")
	}
	for i, p := range sc.plain {
		if p {
			t.Fatalf("action request: PlainFallback true on request %d", i)
		}
	}
}

// Hole 5: an empty reply after a tool ran is still an error, marked partial for Retry.
func TestRunEmptyReplyAfterToolIsError(t *testing.T) {
	dir := t.TempDir()
	_, tp, _ := runScript(t, "list the files in "+dir, listCall(t, dir), &llm.Result{Text: "", Finish: "stop"})
	e := tp.has("error")
	if tp.has("done") != "" || !strings.Contains(e, `"empty":true`) || !strings.Contains(e, `"partial":true`) {
		t.Fatalf("evs %v", tp.evs)
	}
}

func TestActionIntentWider(t *testing.T) {
	for _, s := range []string{"fix the failing test", "please deploy the site", "publish the package", "upload report.pdf to drive", "tweet this: hello", "uninstall node", "build and run the app"} {
		if !actionIntent(s) {
			t.Errorf("want action: %q", s)
		}
	}
	for _, s := range []string{"how do I fix a flat tyre?", "what does deploy mean?", "fix this sentence: me and him goes", "is it worth publishing on npm?"} {
		if actionIntent(s) {
			t.Errorf("not action: %q", s)
		}
	}
	for _, s := range []string{"Published! v1.2 is live on npm.", "I've deployed the site.", "The file has been successfully uploaded."} {
		if !claimsDone(s) {
			t.Errorf("want claim: %q", s)
		}
	}
}
