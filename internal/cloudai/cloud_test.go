package cloudai

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lacksonkapapahue/blaze-desktop/internal/llm"
)

func user(s string) []llm.Message {
	return []llm.Message{{Role: "system", Content: "sys"}, {Role: "user", Content: s}}
}

func tool(name string) []llm.Tool {
	var t llm.Tool
	t.Type = "function"
	t.Function.Name = name
	t.Function.Parameters = map[string]any{"type": "object"}
	return []llm.Tool{t}
}

// A 1x1 PNG on disk, attached as an image.
func pngFile(t *testing.T) string {
	b, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==")
	p := filepath.Join(t.TempDir(), "dot.png")
	if err := os.WriteFile(p, b, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestTaskClassification(t *testing.T) {
	img := pngFile(t)
	withImg := []llm.Message{{Role: "user", Content: "what is this", Attach: []llm.Attach{{Kind: "image", Path: img, Name: "dot.png"}}}}
	cases := []struct {
		name  string
		msgs  []llm.Message
		tools []llm.Tool
		quick bool
		want  string
	}{
		{"image wins over everything", withImg, tool("x"), true, "vision"},
		{"long beats tools", user(strings.Repeat("a", LongChars+10)), tool("x"), false, "long"},
		{"tools are agent", user("hi"), tool("x"), false, "agent"},
		{"code cue", user("Can you refactor this python script and explain why it is written that way? Thanks for the careful help here."), nil, false, "code"},
		{"code fence", user("```go\nfunc main(){}\n```"), nil, false, "code"},
		{"code beats quick call", user("fix this bug"), nil, true, "code"},
		{"short hello is quick", user("hey there"), nil, false, "quick"},
		{"background call is quick", user(strings.Repeat("Tell me about the history of the city in great detail please. ", 3)), nil, true, "quick"},
		{"long normal question is chat", user(strings.Repeat("Tell me about the history of the city in great detail please. ", 3)), nil, false, "chat"},
		{"short but deep is chat", user("prove that root two is irrational"), nil, false, "chat"},
	}
	for _, c := range cases {
		if got := Task(c.msgs, c.tools, c.quick); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
	// A Private image never leaves the computer, so it is not a vision request.
	priv := []llm.Message{{Role: "user", Content: "hey", Attach: []llm.Attach{{Kind: "image", Path: img, Private: true, Desc: "a dot"}}}}
	if got := Task(priv, nil, false); got == "vision" {
		t.Errorf("private image went to vision")
	}
}

func TestThinkLevel(t *testing.T) {
	if Level("hi", 0) != Quick || Level("", 0) != Quick || Level("", 1) != Normal {
		t.Error("empty/greeting levels")
	}
	if Level("explain this", 0) != Deep || Level("2 + 2", 0) != Deep || Level("a? b? c?", 0) != Deep {
		t.Error("deep cues")
	}
	if Level("hello", 2) != Normal {
		t.Error("attachments are normal")
	}
	if Level(strings.Repeat("word ", 30), 0) != Normal {
		t.Error("long plain text is normal")
	}
}

func TestRetryPolicy(t *testing.T) {
	yes := []struct {
		code int
		text string
	}{{0, "timeout"}, {408, ""}, {429, "slow down"}, {500, ""}, {503, "blaze_models_busy"}, {404, ""}, {400, "model_not_found"}}
	for _, c := range yes {
		if !Retryable(c.code, c.text) {
			t.Errorf("%d %q should retry", c.code, c.text)
		}
	}
	no := []struct {
		code int
		text string
	}{{401, "invalid_api_key"}, {403, "waitlisted"}, {429, "quota_exceeded daily"}, {429, "plan allowance used"}, {400, "bad request"}}
	for _, c := range no {
		if Retryable(c.code, c.text) {
			t.Errorf("%d %q should not retry", c.code, c.text)
		}
	}
	if BackoffMs(0, 0) != 1000 || BackoffMs(1, 0) != 3000 || BackoffMs(2, 0) != 7000 {
		t.Error("backoff steps")
	}
	if BackoffMs(0, 5) != 5000 || BackoffMs(0, 21) != 1000 || BackoffMs(1, 20) != 20000 {
		t.Error("Retry-After under 20 s wins")
	}
}

func TestErrorLines(t *testing.T) {
	cases := []struct {
		code int
		text string
		want string
	}{
		{403, "waitlisted You're on the Blaze waiting list", Waitlisted},
		{403, "access_rejected nope", Rejected},
		{403, "forbidden", Expired},
		{401, "invalid_api_key", Expired},
		{429, "quota_exceeded", Allowance},
		{429, "busy", Busy},
		{503, "blaze_models_busy", Busy},
		{404, "", Busy},
		{400, "the model doesn't offer that model", Busy},
		{0, "dial tcp: no such host", Unreachable},
		{400, "something odd", Unreachable},
	}
	for _, c := range cases {
		if got := Friendly(c.code, c.text); got != c.want {
			t.Errorf("Friendly(%d,%q) = %q, want %q", c.code, c.text, got, c.want)
		}
	}
	if Access(401, "waitlisted") != "" || Access(403, "other") != "" {
		t.Error("Access only for 403 waiting-list codes")
	}
	if llm.Plain(&Error{Line: Busy}) != Busy {
		t.Error("Plain must use the cloud line")
	}
}

func TestEmptyReply(t *testing.T) {
	if !IsEmpty(&llm.Result{Text: "<think>hmm</think>  "}) || !IsEmpty(&llm.Result{}) || !IsEmpty(nil) {
		t.Error("reasoning-only is empty")
	}
	if IsEmpty(&llm.Result{Text: "ok"}) || IsEmpty(&llm.Result{ToolCalls: []llm.ToolCall{{ID: "1"}}}) {
		t.Error("text or a tool call is an answer")
	}
	n := WithNudge(user("x"))
	if len(n) != 3 || n[2].Content != Nudge {
		t.Error("nudge appended as a user turn")
	}
}

// Drive, without a network.
func TestDrive(t *testing.T) {
	ctx := context.Background()
	var slept []time.Duration
	sleep := func(_ context.Context, d time.Duration) error { slept = append(slept, d); return nil }

	n := 0
	r, err := Drive(ctx, true, func(a Attempt) (*llm.Result, error) {
		n++
		if n < 3 {
			return nil, &llm.HTTPError{Code: 503, Body: "busy"}
		}
		return &llm.Result{Text: "hi"}, nil
	}, func() string { return "" }, nil, sleep)
	if err != nil || r.Text != "hi" || len(slept) != 2 || slept[0] != time.Second || slept[1] != 3*time.Second {
		t.Fatalf("retries: %v %v %v", r, err, slept)
	}

	n, slept = 0, nil
	_, err = Drive(ctx, true, func(a Attempt) (*llm.Result, error) { n++; return nil, &llm.HTTPError{Code: 500, Body: "x"} }, func() string { return "" }, nil, sleep)
	if n != MaxTries || err == nil || err.Error() != Busy {
		t.Fatalf("give up: n=%d err=%v", n, err)
	}

	for _, c := range []struct {
		he   *llm.HTTPError
		want string
	}{{&llm.HTTPError{Code: 401, Body: "invalid_api_key"}, Expired}, {&llm.HTTPError{Code: 403, Body: "waitlisted"}, Waitlisted}, {&llm.HTTPError{Code: 429, Body: "quota_exceeded"}, Allowance}} {
		n = 0
		_, err = Drive(ctx, true, func(a Attempt) (*llm.Result, error) { n++; return nil, c.he }, func() string { return "" }, nil, sleep)
		if n != 1 || err == nil || err.Error() != c.want {
			t.Errorf("%v: n=%d err=%v", c.he, n, err)
		}
	}

	var tries []Attempt
	_, err = Drive(ctx, true, func(a Attempt) (*llm.Result, error) {
		tries = append(tries, a)
		return &llm.Result{Text: "<think>..</think>"}, nil
	},
		func() string { return "groq/llama" }, nil, sleep)
	if err == nil || err.Error() != EmptyFailed || len(tries) != 3 {
		t.Fatalf("empties: %v %d", err, len(tries))
	}
	if !tries[0].Stream || tries[0].Nudge || tries[1].Stream || !tries[1].Nudge || !tries[2].Stream || len(tries[2].Avoid) != 1 || tries[2].Avoid[0] != "groq/llama" {
		t.Errorf("empty-reply attempts: %+v", tries)
	}

	tries = nil
	_, _ = Drive(ctx, true, func(a Attempt) (*llm.Result, error) {
		tries = append(tries, a)
		if len(tries) == 1 {
			return nil, &llm.HTTPError{Code: 0, Body: "stopped"}
		}
		return &llm.Result{Text: "ok"}, nil
	}, func() string { return "cf/llama" }, nil, sleep)
	if len(tries) != 2 || len(tries[1].Avoid) != 1 {
		t.Errorf("avoid after a broken stream: %+v", tries)
	}

	trims := 0
	n = 0
	r, err = Drive(ctx, false, func(a Attempt) (*llm.Result, error) {
		n++
		if n == 1 {
			return nil, &llm.HTTPError{Code: 413, Body: "too large"}
		}
		return &llm.Result{Text: "fits"}, nil
	}, func() string { return "" }, func(err error) bool { trims++; return trims == 1 && llm.TooLarge(err) }, sleep)
	if err != nil || r.Text != "fits" || trims != 1 {
		t.Errorf("trim: %v %v %d", r, err, trims)
	}

	cctx, cancel := context.WithCancel(ctx)
	cancel()
	_, err = Drive(cctx, true, func(a Attempt) (*llm.Result, error) { return nil, &llm.HTTPError{Code: 0, Body: "x"} }, func() string { return "" }, nil, sleep)
	if err != context.Canceled {
		t.Errorf("cancel: %v", err)
	}
}
