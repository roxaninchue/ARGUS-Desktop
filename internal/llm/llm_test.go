package llm

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/lacksonkapapahue/argus-desktop/internal/config"
)

func sse(chunks ...string) string {
	var b strings.Builder
	for _, c := range chunks {
		b.WriteString("data: " + c + "\n\n")
	}
	b.WriteString("data: [DONE]\n\n")
	return b.String()
}

func parse(t *testing.T, body string) *Result {
	t.Helper()
	r, err := ParseStream(strings.NewReader(body), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

var known = []string{"file_read", "file_edit", "file_write", "file_list", "run_shell", "git"}

// Gemini's OpenAI layer sends each parallel call whole, in its own chunk, all with index 0.
func TestGeminiWholeCallsSameIndex(t *testing.T) {
	r := parse(t, sse(
		`{"choices":[{"delta":{"role":"assistant","tool_calls":[{"index":0,"id":"a1","type":"function","function":{"name":"file_read","arguments":"{\"path\":\"index.html\"}"}}]}}]}`,
		`{"choices":[{"delta":{"tool_calls":[{"index":0,"id":"a2","type":"function","function":{"name":"file_edit","arguments":"{\"path\":\"index.html\",\"old\":\"a\",\"new\":\"b\"}"}}]}}]}`,
	))
	if len(r.ToolCalls) != 2 {
		t.Fatalf("want 2 calls, got %d: %+v", len(r.ToolCalls), r.ToolCalls)
	}
	if r.ToolCalls[0].Function.Name != "file_read" || r.ToolCalls[1].Function.Name != "file_edit" {
		t.Fatalf("names glued or lost: %+v", r.ToolCalls)
	}
	for _, c := range r.ToolCalls {
		if !completeJSON(c.Function.Arguments) {
			t.Fatalf("args not valid JSON: %q", c.Function.Arguments)
		}
	}
}

// No index and no id at all, still one whole call per chunk.
func TestWholeCallsNoIndexNoID(t *testing.T) {
	r := parse(t, sse(
		`{"choices":[{"delta":{"tool_calls":[{"type":"function","function":{"name":"file_list","arguments":"{}"}}]}}]}`,
		`{"choices":[{"delta":{"tool_calls":[{"type":"function","function":{"name":"run_shell","arguments":"{\"command\":\"ls\"}"}}]}}]}`,
	))
	if len(r.ToolCalls) != 2 || r.ToolCalls[0].Function.Name != "file_list" || r.ToolCalls[1].Function.Name != "run_shell" {
		t.Fatalf("got %+v", r.ToolCalls)
	}
	if r.ToolCalls[0].ID == r.ToolCalls[1].ID || r.ToolCalls[0].ID == "" {
		t.Fatalf("ids must be unique and set: %+v", r.ToolCalls)
	}
}

// Arguments as a JSON object instead of a string used to make the whole chunk fail to
// decode, silently dropping the call's name and leaving argument fragments nameless.
func TestArgumentsAsObject(t *testing.T) {
	r := parse(t, sse(
		`{"choices":[{"delta":{"tool_calls":[{"index":0,"id":"x","function":{"name":"file_write","arguments":{"path":"a.html","content":"<h1>Hi</h1>"}}}]}}]}`,
	))
	if len(r.ToolCalls) != 1 || r.ToolCalls[0].Function.Name != "file_write" {
		t.Fatalf("got %+v", r.ToolCalls)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(r.ToolCalls[0].Function.Arguments), &m); err != nil || m["path"] != "a.html" {
		t.Fatalf("args %q: %v", r.ToolCalls[0].Function.Arguments, err)
	}
}

// Classic OpenAI streaming: id+name once, then argument fragments under the same index,
// two calls interleaved by index.
func TestOpenAIFragmentedInterleaved(t *testing.T) {
	r := parse(t, sse(
		`{"choices":[{"delta":{"tool_calls":[{"index":0,"id":"c0","type":"function","function":{"name":"file_read","arguments":""}}]}}]}`,
		`{"choices":[{"delta":{"tool_calls":[{"index":1,"id":"c1","type":"function","function":{"name":"git","arguments":""}}]}}]}`,
		`{"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"pa"}}]}}]}`,
		`{"choices":[{"delta":{"tool_calls":[{"index":1,"function":{"arguments":"{\"args\":\"status\"}"}}]}}]}`,
		`{"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"th\":\"x\"}"}}]}}]}`,
	))
	if len(r.ToolCalls) != 2 {
		t.Fatalf("got %+v", r.ToolCalls)
	}
	if r.ToolCalls[0].Function.Arguments != `{"path":"x"}` || r.ToolCalls[1].Function.Arguments != `{"args":"status"}` {
		t.Fatalf("args wrong: %+v", r.ToolCalls)
	}
}

// A provider that repeats the full name on every fragment must not get "file_readfile_read".
func TestRepeatedNameOnFragments(t *testing.T) {
	r := parse(t, sse(
		`{"choices":[{"delta":{"tool_calls":[{"index":0,"id":"c0","function":{"name":"file_read","arguments":"{\"path\":"}}]}}]}`,
		`{"choices":[{"delta":{"tool_calls":[{"index":0,"id":"c0","function":{"name":"file_read","arguments":"\"a\"}"}}]}}]}`,
	))
	if len(r.ToolCalls) != 1 || r.ToolCalls[0].Function.Name != "file_read" || r.ToolCalls[0].Function.Arguments != `{"path":"a"}` {
		t.Fatalf("got %+v", r.ToolCalls)
	}
}

// A nameless fragment at a fresh index (its header was lost) must not become a call with an empty name.
func TestNamelessFragmentDropped(t *testing.T) {
	r := parse(t, sse(
		`{"choices":[{"delta":{"tool_calls":[{"index":0,"id":"c0","function":{"name":"file_list","arguments":"{}"}}]}}]}`,
		`{"choices":[{"delta":{"tool_calls":[{"index":3,"function":{"arguments":"{\"path\":\"x\"}"}}]}}]}`,
	))
	for _, c := range r.ToolCalls {
		if c.Function.Name == "" {
			t.Fatalf("empty-name call survived: %+v", r.ToolCalls)
		}
	}
	if len(r.ToolCalls) != 1 {
		t.Fatalf("got %+v", r.ToolCalls)
	}
}

// Gemini 3 thought signatures must survive the round trip.
func TestExtraContentKept(t *testing.T) {
	r := parse(t, sse(
		`{"choices":[{"delta":{"tool_calls":[{"index":0,"id":"g","function":{"name":"file_list","arguments":"{}"},"extra_content":{"google":{"thought_signature":"abc"}}}]}}]}`,
	))
	if len(r.ToolCalls) != 1 || !strings.Contains(string(r.ToolCalls[0].ExtraContent), "thought_signature") {
		t.Fatalf("extra_content lost: %+v", r.ToolCalls)
	}
	b, _ := json.Marshal(Message{Role: "assistant", ToolCalls: r.ToolCalls})
	if !strings.Contains(string(b), `"extra_content":{"google":{"thought_signature":"abc"}}`) {
		t.Fatalf("not re-sent: %s", b)
	}
}

func TestStreamError(t *testing.T) {
	_, err := ParseStream(strings.NewReader(sse(`{"error":{"code":429,"message":"Resource has been exhausted"}}`)), nil, nil)
	var he *HTTPError
	if !errors.As(err, &he) || he.Code != 429 {
		t.Fatalf("got %v", err)
	}
}

func TestParseArgsRepair(t *testing.T) {
	cases := map[string]string{
		`{"path":"a",}`:                  "a",
		`{path: 'a'}`:                    "a",
		`{"path":"a"}{"path":"b"}`:       "a",
		`{"path":"a"`:                    "a",
		`{"path":"a", "n": [1, 2`:        "a",
		"```json\n{\"path\":\"a\"}\n```": "a",
		`"{\"path\":\"a\"}"`:             "a",
	}
	for in, want := range cases {
		m, ok := ParseArgs(in)
		if !ok || m["path"] != want {
			t.Errorf("ParseArgs(%q) = %v, %v", in, m, ok)
		}
	}
	for _, bad := range []string{`not json at all`, `{"path":"a`, `{"path":"x","content":"<h1>Half a fi`, `{"path":`} {
		if _, ok := ParseArgs(bad); ok {
			t.Errorf("unsafe arguments accepted: %q", bad)
		}
	}
}

func TestNormalizeCalls(t *testing.T) {
	mk := func(id, name, args string) ToolCall {
		var c ToolCall
		c.ID, c.Type = id, "function"
		c.Function.Name, c.Function.Arguments = name, args
		return c
	}
	n := NormalizeCalls([]ToolCall{
		mk("1", "", `{}`), // no name: dropped
		mk("2", "functions.file_read", `{"path":"a"}`),      // prefix: resolved
		mk("3", "file_edit", `this is not json`),            // unreadable args: dropped
		mk("4", "file_listrun_shell", `{}{"command":"ls"}`), // glued: split
		mk("5", "make_coffee", `{}`),                        // unknown: kept, answered with an error
	}, known)
	if len(n.Dropped) != 2 {
		t.Fatalf("dropped %v", n.Dropped)
	}
	var names []string
	for _, c := range n.Calls {
		names = append(names, c.Function.Name)
		if c.Function.Name == "" || !completeJSON(c.Function.Arguments) {
			t.Fatalf("unsafe call kept: %+v", c)
		}
	}
	if strings.Join(names, ",") != "file_read,file_list,run_shell,make_coffee" {
		t.Fatalf("names %v", names)
	}
	if len(n.Unknown) != 1 || n.Unknown[0] != "make_coffee" {
		t.Fatalf("unknown %v", n.Unknown)
	}
}

// History saved by v0.2 with the bad calls must come out clean for the next request.
func TestSanitizeBadHistory(t *testing.T) {
	bad := ToolCall{ID: "e1", Type: "function"}
	bad.Function.Arguments = `{"path":"x"}`
	trunc := ToolCall{ID: "e2", Type: "function"}
	trunc.Function.Name, trunc.Function.Arguments = "file_edit", `{"path":"index.html","old":"<h1>`
	broken := ToolCall{ID: "e3", Type: "function"}
	broken.Function.Name, broken.Function.Arguments = "file_write", `]]]`
	good := ToolCall{ID: "g1", Type: "function"}
	good.Function.Name, good.Function.Arguments = "file_list", ``
	msgs := []Message{
		{Role: "user", Content: "make it blue"},
		{Role: "assistant", ToolCalls: []ToolCall{bad, trunc, broken, good}},
		{Role: "tool", ToolCallID: "e1", Content: "Error: there is no tool called . Use one of the listed tools."},
		{Role: "tool", ToolCallID: "e2", Content: "Error in file_edit: the arguments weren't valid JSON"},
		{Role: "tool", ToolCallID: "e3", Content: "Error"},
		// g1's reply is missing entirely
		{Role: "tool", ToolCallID: "zzz", Content: "orphan"},
		{Role: "assistant", Content: ""},
	}
	out := Sanitize(msgs)
	for i, m := range out {
		for _, c := range m.ToolCalls {
			if c.Function.Name == "" || !completeJSON(c.Function.Arguments) {
				t.Fatalf("unsafe call in history: %+v", c)
			}
		}
		if m.Role == "tool" {
			prev := out[i-1]
			if prev.Role != "assistant" && prev.Role != "tool" {
				t.Fatalf("tool message not after its call at %d", i)
			}
		}
		if m.Role == "assistant" && len(m.ToolCalls) == 0 && strings.TrimSpace(m.Content) == "" {
			t.Fatalf("empty assistant turn at %d", i)
		}
	}
	// Every kept call is answered exactly once, right after it.
	a := out[1]
	if len(a.ToolCalls) != 1 || a.ToolCalls[0].ID != "g1" { // only the good call survives
		t.Fatalf("calls kept: %+v", a.ToolCalls)
	}
	for j, c := range a.ToolCalls {
		if out[2+j].Role != "tool" || out[2+j].ToolCallID != c.ID {
			t.Fatalf("call %s not answered in order: %+v", c.ID, out[2+j])
		}
	}
	b, _ := json.Marshal(out)
	if strings.Contains(string(b), `"name":""`) {
		t.Fatalf("empty name went out: %s", b)
	}
}

func TestFlattenHasNoToolTurns(t *testing.T) {
	c := ToolCall{ID: "a", Type: "function"}
	c.Function.Name, c.Function.Arguments = "file_list", "{}"
	out := Flatten([]Message{{Role: "user", Content: "hi"}, {Role: "assistant", ToolCalls: []ToolCall{c}}, {Role: "tool", ToolCallID: "a", Name: "file_list", Content: "x"}})
	for _, m := range out {
		if m.Role == "tool" || len(m.ToolCalls) > 0 {
			t.Fatalf("tool turn left: %+v", out)
		}
	}
}

func TestGeminiSchemaScrub(t *testing.T) {
	var tl Tool
	tl.Function.Name = "x"
	tl.Function.Parameters = map[string]any{"type": "object", "additionalProperties": false, "properties": map[string]any{
		"format": map[string]any{"type": "string", "format": "uri", "default": "a"},
		"n":      map[string]any{"type": "integer", "enum": []any{1, 2}},
	}}
	out := AdaptTools([]Tool{tl}, "gemini")[0].Function.Parameters
	b, _ := json.Marshal(out)
	s := string(b)
	if strings.Contains(s, "additionalProperties") || strings.Contains(s, `"uri"`) || strings.Contains(s, "default") || !strings.Contains(s, `"format":{`) || !strings.Contains(s, `["1","2"]`) {
		t.Fatalf("scrub: %s", s)
	}
	if _, ok := tl.Function.Parameters["additionalProperties"]; !ok {
		t.Fatalf("original mutated")
	}
}

func TestToolCapability(t *testing.T) {
	if CanUseTools(config.KeylessID, "openai-fast") {
		t.Fatal("keyless must never be used for tools")
	}
	if !CanUseTools("gemini", "gemini-3.8-flash") {
		t.Fatal("gemini default should use tools")
	}
	if CanUseTools("gemini", "gemma-3-27b-it") {
		t.Fatal("gemma can't call functions on the Gemini API")
	}
}

func TestPlainErrors(t *testing.T) {
	for raw, want := range map[string]string{
		"HTTP 429: Resource has been exhausted (e.g. check quota).": "free limit",
		"HTTP 400: Request contains an invalid argument.":           "format",
		"HTTP 401: invalid api key":                                 "key",
		"HTTP 503: overloaded":                                      "trouble",
	} {
		if got := PlainText("Google Gemini", 0, raw); !strings.Contains(got, want) {
			t.Errorf("%q -> %q", raw, got)
		}
	}
}

func TestBudgetHints(t *testing.T) {
	if d, ok := HintFrom(`Please retry in 1m3s`); !ok || d.Seconds() != 63 {
		t.Fatalf("got %v %v", d, ok)
	}
	if d, ok := HintFrom(`"retryDelay": "40s"`); !ok || d.Seconds() != 40 {
		t.Fatalf("got %v %v", d, ok)
	}
}

// End to end against a fake Gemini: the first turn streams the broken shapes; the
// second request must carry only clean history (Gemini answers 400 otherwise).
func TestRouterNeverSendsBrokenHistory(t *testing.T) {
	home := t.TempDir()
	os.Setenv("ARGUS_HOME", home)
	defer os.Unsetenv("ARGUS_HOME")
	os.Setenv("GROQ_API_KEY", "gsk_test")
	defer os.Unsetenv("GROQ_API_KEY")
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var body struct {
			Messages []Message `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		for _, m := range body.Messages {
			for _, c := range m.ToolCalls {
				if c.Function.Name == "" || !completeJSON(c.Function.Arguments) {
					w.WriteHeader(400)
					w.Write([]byte(`{"error":{"code":400,"message":"Request contains an invalid argument.","status":"INVALID_ARGUMENT"}}`))
					return
				}
			}
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte(sse(`{"choices":[{"delta":{"content":"done"}}]}`)))
	}))
	defer srv.Close()
	saved := config.Providers
	defer func() { config.Providers = saved }()
	for i := range config.Providers {
		if config.Providers[i].ID == "groq" {
			config.Providers[i].BaseURL = srv.URL
		}
	}
	cfg := config.Load()
	cfg.Chain = []config.Entry{{Provider: "groq", Model: "llama-3.3-70b-versatile"}}
	bad := ToolCall{ID: "e1", Type: "function"}
	bad.Function.Arguments = `{"path":`
	hist := []Message{{Role: "user", Content: "hi"}, {Role: "assistant", ToolCalls: []ToolCall{bad}}, {Role: "tool", ToolCallID: "e1", Content: "Error"}}
	var tl Tool
	tl.Type = "function"
	tl.Function.Name = "file_list"
	tl.Function.Parameters = map[string]any{"type": "object", "properties": map[string]any{}}
	r, err := CompleteWith(context.Background(), cfg, hist, []Tool{tl}, Opts{})
	if err != nil {
		t.Fatalf("router sent broken history: %v", err)
	}
	if r.Text != "done" || calls != 1 {
		t.Fatalf("text %q after %d calls", r.Text, calls)
	}
}

// Pollinations is never used for a tool turn, even when nothing else is set up.
func TestKeylessNeverForTools(t *testing.T) {
	home := t.TempDir()
	os.Setenv("ARGUS_HOME", home)
	defer os.Unsetenv("ARGUS_HOME")
	cfg := config.Load()
	cfg.Chain = []config.Entry{{Provider: config.KeylessID, Model: "openai-fast"}}
	var tl Tool
	tl.Function.Name = "file_list"
	for _, tg := range Targets(cfg, true) {
		if tg.Provider == config.KeylessID {
			t.Fatal("keyless in a tool route")
		}
	}
	_, err := CompleteWith(context.Background(), cfg, []Message{{Role: "user", Content: "hi"}}, []Tool{tl}, Opts{})
	var re *RouteError
	if !errors.As(err, &re) || !re.NoModels {
		t.Fatalf("want a 'no model set up' error, got %v", err)
	}
}
