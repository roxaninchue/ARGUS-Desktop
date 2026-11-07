package router

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lacksonkapapahue/argus-desktop/internal/config"
	"github.com/lacksonkapapahue/argus-desktop/internal/keystore"
	"github.com/lacksonkapapahue/argus-desktop/internal/llm"
	"github.com/zalando/go-keyring"
)

type memKC struct {
	mu sync.Mutex
	m  map[string]string
}

func (f *memKC) Get(a string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if v, ok := f.m[a]; ok {
		return v, nil
	}
	return "", keyring.ErrNotFound
}
func (f *memKC) Set(a, v string) error { f.mu.Lock(); f.m[a] = v; f.mu.Unlock(); return nil }
func (f *memKC) Delete(a string) error { f.mu.Lock(); delete(f.m, a); f.mu.Unlock(); return nil }

var provN atomic.Int32

// fakeProvider registers a provider served by h, with a key in an env var.
func fakeProvider(t *testing.T, h http.HandlerFunc) (string, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	id := fmt.Sprintf("fake-%d", provN.Add(1))
	env := strings.ToUpper(strings.ReplaceAll(id, "-", "_")) + "_KEY"
	t.Setenv(env, "test-key-"+id)
	config.Register(config.Provider{ID: id, Name: "Fake " + id, BaseURL: srv.URL + "/v1/", DefaultModel: "m1", Env: env, Free: true})
	return id, srv
}

func newTestRouter(t *testing.T, chain ...string) (*Router, *config.Config) {
	t.Helper()
	t.Setenv("ARGUS_HOME", t.TempDir())
	keystore.Keychain = &memKC{m: map[string]string{}}
	retryBackoff = time.Millisecond
	cfg := config.Load()
	for _, id := range chain {
		cfg.Chain = append(cfg.Chain, config.Entry{Provider: id, Model: "m1"})
	}
	r := New(Options{Config: func() *config.Config { return cfg }, Version: "test"})
	return r, cfg
}

func okJSON(w http.ResponseWriter, text string) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"id":"x","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":%q},"finish_reason":"stop"}],"usage":{"prompt_tokens":11,"completion_tokens":3}}`, text)
}

func post(t *testing.T, r *Router, path, key string, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "http://127.0.0.1:20128"+path, strings.NewReader(string(b)))
	req.Host = "127.0.0.1:20128"
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	rec := httptest.NewRecorder()
	r.Handler().ServeHTTP(rec, req)
	return rec
}

func chatBody(model string, stream bool) map[string]any {
	return map[string]any{"model": model, "stream": stream, "messages": []any{map[string]any{"role": "user", "content": "hi"}}}
}

func TestFallbackOnRateLimit(t *testing.T) {
	a, _ := fakeProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(429)
		io.WriteString(w, `{"error":{"message":"rate limit reached"}}`)
	})
	b, _ := fakeProvider(t, func(w http.ResponseWriter, r *http.Request) { okJSON(w, "from b") })
	r, _ := newTestRouter(t, a, b)
	rec := post(t, r, "/v1/chat/completions", r.APIKey(), chatBody("auto", false))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "from b") {
		t.Fatalf("want b's answer, got %d %s", rec.Code, rec.Body)
	}
	if got := rec.Header().Get("X-ArgusRouter-Provider"); got != b {
		t.Fatalf("answered by %q, want %q", got, b)
	}
	if llm.CoolingFor(a, "m1") <= 0 {
		t.Fatal("the rate-limited provider should rest")
	}
	// Next request skips the resting one without calling it.
	pl, _ := r.plan("auto", false)
	if pl.Targets[0].Provider != b {
		t.Fatalf("resting provider should go last, order %v", pl.Targets)
	}
	e := r.log.recent(1)[0]
	if e.Provider != b || e.Fallbacks != 1 || e.In != 11 || e.Out != 3 {
		t.Fatalf("log entry: %+v", e)
	}
}

func TestRetryOnOutage(t *testing.T) {
	var n atomic.Int32
	a, _ := fakeProvider(t, func(w http.ResponseWriter, r *http.Request) {
		if n.Add(1) == 1 {
			w.WriteHeader(503)
			io.WriteString(w, `{"error":{"message":"overloaded"}}`)
			return
		}
		okJSON(w, "second try")
	})
	r, _ := newTestRouter(t, a)
	rec := post(t, r, "/v1/chat/completions", r.APIKey(), chatBody("auto", false))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "second try") || n.Load() != 2 {
		t.Fatalf("want one retry: %d calls, %d %s", n.Load(), rec.Code, rec.Body)
	}
}

func TestAllFailReturnsError(t *testing.T) {
	a, _ := fakeProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		io.WriteString(w, `{"error":"bad key"}`)
	})
	r, _ := newTestRouter(t, a)
	rec := post(t, r, "/v1/chat/completions", r.APIKey(), chatBody("auto", false))
	if rec.Code != 502 || !strings.Contains(rec.Body.String(), "bad key") {
		t.Fatalf("got %d %s", rec.Code, rec.Body)
	}
	if e := r.log.recent(1)[0]; e.Status != 502 || len(e.Route) != 1 {
		t.Fatalf("failed request should be logged with its route: %+v", e)
	}
}

func TestSSEPassthrough(t *testing.T) {
	chunks := []string{
		`{"id":"c","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"role":"assistant","content":"Hel"}}]}`,
		`{"id":"c","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"lo"}}]}`,
		`{"id":"c","object":"chat.completion.chunk","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":7,"completion_tokens":2}}`,
	}
	var got map[string]any
	a, _ := fakeProvider(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.Header().Set("Content-Type", "text/event-stream")
		for _, c := range chunks {
			fmt.Fprintf(w, "data: %s\n\n", c)
			w.(http.Flusher).Flush()
		}
		io.WriteString(w, "data: [DONE]\n\n")
	})
	r, _ := newTestRouter(t, a)
	srv := httptest.NewServer(r.Handler())
	defer srv.Close()
	b, _ := json.Marshal(chatBody(a+"/m1", true))
	req, _ := http.NewRequest("POST", srv.URL+"/v1/chat/completions", strings.NewReader(string(b)))
	req.Header.Set("Authorization", "Bearer "+r.APIKey())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if !strings.Contains(resp.Header.Get("Content-Type"), "event-stream") {
		t.Fatalf("content type %q", resp.Header.Get("Content-Type"))
	}
	var data []string
	sc := bufio.NewScanner(resp.Body)
	for sc.Scan() {
		if l := sc.Text(); strings.HasPrefix(l, "data: ") {
			data = append(data, strings.TrimPrefix(l, "data: "))
		}
	}
	want := append(append([]string{}, chunks...), "[DONE]")
	if strings.Join(data, "\n") != strings.Join(want, "\n") {
		t.Fatalf("stream changed in transit:\n%v", data)
	}
	if got["model"] != "m1" || got["stream"] != true {
		t.Fatalf("upstream body: %v", got)
	}
	time.Sleep(20 * time.Millisecond)
	if e := r.log.recent(1)[0]; !e.Stream || e.In != 7 || e.Out != 2 || e.Provider != a {
		t.Fatalf("log: %+v", e)
	}
}

func TestStreamErrorInFirstEventFallsBack(t *testing.T) {
	a, _ := fakeProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, ": OPENROUTER PROCESSING\n\ndata: {\"error\":{\"message\":\"Provider returned error\",\"code\":429}}\n\n")
	})
	b, _ := fakeProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\ndata: [DONE]\n\n")
	})
	r, _ := newTestRouter(t, a, b)
	rec := post(t, r, "/v1/chat/completions", r.APIKey(), chatBody("auto", true))
	if rec.Header().Get("X-ArgusRouter-Provider") != b || !strings.Contains(rec.Body.String(), `"ok"`) {
		t.Fatalf("got %s: %s", rec.Header().Get("X-ArgusRouter-Provider"), rec.Body)
	}
}

func TestKeyRingRestsAKey(t *testing.T) {
	var keys []string
	var mu sync.Mutex
	a, _ := fakeProvider(t, func(w http.ResponseWriter, r *http.Request) {
		k := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		mu.Lock()
		keys = append(keys, k)
		mu.Unlock()
		if k != "second" {
			w.WriteHeader(429)
			io.WriteString(w, `{"error":{"message":"quota exceeded for today"}}`)
			return
		}
		okJSON(w, "second key")
	})
	r, cfg := newTestRouter(t, a)
	if _, err := config.Keys().SetExtras(a, []string{"second"}); err != nil {
		t.Fatal(err)
	}
	if len(cfg.AllKeys(a)) != 2 {
		t.Fatalf("keys %v", cfg.AllKeys(a))
	}
	rec := post(t, r, "/v1/chat/completions", r.APIKey(), chatBody("auto", false))
	if rec.Code != 200 || len(keys) != 2 || keys[1] != "second" {
		t.Fatalf("want the second key to carry on: %v %d %s", keys, rec.Code, rec.Body)
	}
	if !llm.KeyResting(a, "m1", "test-key-"+a) {
		t.Fatal("the first key should rest")
	}
}

func TestStrategies(t *testing.T) {
	h := func(w http.ResponseWriter, r *http.Request) { okJSON(w, "x") }
	a, _ := fakeProvider(t, h)
	b, _ := fakeProvider(t, h)
	c, _ := fakeProvider(t, h)
	r, _ := newTestRouter(t)
	mk := func(s string) Combo {
		return Combo{Name: "c " + s, Strategy: s, Targets: []config.Entry{{Provider: a, Model: "m1"}, {Provider: b, Model: "m1"}, {Provider: c, Model: "m1"}}}
	}
	first := func(id string) string {
		pl, err := r.plan(id, false)
		if err != nil || len(pl.Targets) != 3 {
			t.Fatalf("plan %s: %v %v", id, pl.Targets, err)
		}
		return pl.Targets[0].Provider
	}
	pc, _ := r.SaveCombo(mk(StratPriority))
	if first(pc.ID) != a {
		t.Fatal("priority keeps order")
	}
	rc, _ := r.SaveCombo(mk(StratRoundRobin))
	if x, y, z := first(rc.ID), first(rc.ID), first(rc.ID); x == y || y == z || x == z {
		t.Fatalf("round robin should rotate: %s %s %s", x, y, z)
	}
	llm.ReportOK(a, "m1", 900)
	llm.ReportOK(b, "m1", 120)
	llm.ReportOK(c, "m1", 400)
	lc, _ := r.SaveCombo(mk(StratLeastLatency))
	if first(lc.ID) != b {
		t.Fatal("least latency should pick the fastest")
	}
	// The circuit breaker overrides any strategy: a cooling model goes last.
	llm.ReportFail(b, "m1", &llm.HTTPError{Code: 500, Body: "down"})
	pl, _ := r.plan(lc.ID, false)
	if pl.Targets[2].Provider != b {
		t.Fatalf("cooling target should be last: %v", pl.Targets)
	}
	// By name works too, and "auto" isn't allowed as a combo name.
	if _, err := r.plan("C priority", false); err != nil {
		t.Fatal("combo by name:", err)
	}
	if _, err := r.SaveCombo(Combo{Name: "auto"}); err == nil {
		t.Fatal("auto is reserved")
	}
	if err := r.DeleteCombo(pc.ID); err != nil || r.Settings().combo(pc.ID) != nil {
		t.Fatal("delete")
	}
}

func TestQuotaAwareAndCheapest(t *testing.T) {
	h := func(w http.ResponseWriter, r *http.Request) { okJSON(w, "x") }
	a, _ := fakeProvider(t, h)
	b, _ := fakeProvider(t, h)
	r, _ := newTestRouter(t)
	free[a] = FreeInfo{Type: "recurring-daily", Monthly: 1_000_000}
	free[b] = FreeInfo{Type: "recurring-monthly", Monthly: 900_000_000}
	q, _ := r.SaveCombo(Combo{Name: "q", Strategy: StratQuotaAware, Targets: []config.Entry{{Provider: a, Model: "m1"}, {Provider: b, Model: "m1"}}})
	pl, _ := r.plan(q.ID, false)
	if pl.Targets[0].Provider != b {
		t.Fatalf("quota-aware should prefer the bigger allowance: %v", pl.Targets)
	}
	ch, _ := r.SaveCombo(Combo{Name: "ch", Strategy: StratCheapest, Targets: []config.Entry{{Provider: "openai", Model: "gpt-4o"}, {Provider: a, Model: "m1"}}})
	t.Setenv("OPENAI_API_KEY", "sk-test")
	pl, _ = r.plan(ch.ID, false)
	if len(pl.Targets) != 2 || pl.Targets[0].Provider != a {
		t.Fatalf("cheapest should put the free tier first: %v", pl.Targets)
	}
}

func TestAuthAndHost(t *testing.T) {
	r, _ := newTestRouter(t)
	if rec := post(t, r, "/v1/chat/completions", "", chatBody("auto", false)); rec.Code != 401 {
		t.Fatalf("no key: %d", rec.Code)
	}
	if rec := post(t, r, "/v1/chat/completions", "br-wrong", chatBody("auto", false)); rec.Code != 401 {
		t.Fatalf("wrong key: %d", rec.Code)
	}
	req := httptest.NewRequest("GET", "http://evil.example/v1/models", nil)
	req.Header.Set("Authorization", "Bearer "+r.APIKey())
	rec := httptest.NewRecorder()
	r.Handler().ServeHTTP(rec, req)
	if rec.Code != 403 {
		t.Fatalf("a rebinding host must be refused: %d", rec.Code)
	}
	req = httptest.NewRequest("GET", "http://127.0.0.1:20128/v1/models", nil)
	req.Host = "127.0.0.1:20128"
	req.Header.Set("x-api-key", r.APIKey())
	rec = httptest.NewRecorder()
	r.Handler().ServeHTTP(rec, req)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"auto"`) {
		t.Fatalf("models: %d %s", rec.Code, rec.Body)
	}
	if !strings.HasPrefix(r.APIKey(), "br-") || len(r.APIKey()) != 43 {
		t.Fatalf("key %q", r.APIKey())
	}
	// The dashboard API takes the session token.
	req = httptest.NewRequest("GET", "http://localhost:20128/api/state", nil)
	req.Host = "localhost:20128"
	req.Header.Set("X-Argus-Token", r.Token())
	rec = httptest.NewRecorder()
	r.Handler().ServeHTTP(rec, req)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"strategies"`) {
		t.Fatalf("state: %d %s", rec.Code, rec.Body)
	}
	// The same key survives a restart of the app.
	r2 := New(Options{Config: func() *config.Config { return config.Load() }})
	if r2.APIKey() != r.APIKey() {
		t.Fatal("key should persist")
	}
}

func TestAnthropicTranslation(t *testing.T) {
	var got map[string]any
	a, _ := fakeProvider(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"choices":[{"message":{"role":"assistant","content":"Let me look.","tool_calls":[{"id":"t1","type":"function","function":{"name":"read_file","arguments":"{\"path\":\"a.go\"}"}}]},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":20,"completion_tokens":9}}`)
	})
	r, _ := newTestRouter(t, a)
	in := map[string]any{"model": "claude-sonnet-4-5", "max_tokens": 100, "system": "Be brief.",
		"tools": []any{map[string]any{"name": "read_file", "description": "Read", "input_schema": map[string]any{"type": "object", "properties": map[string]any{"path": map[string]any{"type": "string"}}}}},
		"messages": []any{
			map[string]any{"role": "user", "content": "open a.go"},
			map[string]any{"role": "assistant", "content": []any{map[string]any{"type": "tool_use", "id": "u1", "name": "read_file", "input": map[string]any{"path": "b.go"}}}},
			map[string]any{"role": "user", "content": []any{map[string]any{"type": "tool_result", "tool_use_id": "u1", "content": "package b"}}},
		}}
	rec := post(t, r, "/v1/messages", r.APIKey(), in)
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	msgs := got["messages"].([]any)
	if len(msgs) != 4 || msgs[0].(map[string]any)["role"] != "system" || msgs[3].(map[string]any)["role"] != "tool" || msgs[3].(map[string]any)["tool_call_id"] != "u1" {
		t.Fatalf("upstream messages: %v", msgs)
	}
	if calls := msgs[2].(map[string]any)["tool_calls"].([]any); len(calls) != 1 {
		t.Fatalf("assistant tool call lost: %v", msgs[2])
	}
	if tools := got["tools"].([]any); tools[0].(map[string]any)["type"] != "function" {
		t.Fatalf("tools: %v", tools)
	}
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	content := out["content"].([]any)
	if out["type"] != "message" || out["stop_reason"] != "tool_use" || len(content) != 2 || content[1].(map[string]any)["type"] != "tool_use" ||
		content[1].(map[string]any)["input"].(map[string]any)["path"] != "a.go" || out["model"] != "claude-sonnet-4-5" {
		t.Fatalf("anthropic answer: %s", rec.Body)
	}
	// Streamed.
	in["stream"] = true
	rec = post(t, r, "/v1/messages", r.APIKey(), in)
	body := rec.Body.String()
	for _, ev := range []string{"event: message_start", "event: content_block_start", `"text_delta"`, `"input_json_delta"`, "event: message_delta", "event: message_stop"} {
		if !strings.Contains(body, ev) {
			t.Fatalf("stream lacks %s:\n%s", ev, body)
		}
	}
}

func TestGeminiTranslation(t *testing.T) {
	var got map[string]any
	a, _ := fakeProvider(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		okJSON(w, "Hi from upstream")
	})
	r, _ := newTestRouter(t, a)
	in := map[string]any{"systemInstruction": map[string]any{"parts": []any{map[string]any{"text": "sys"}}},
		"contents":         []any{map[string]any{"role": "user", "parts": []any{map[string]any{"text": "hello"}}}},
		"generationConfig": map[string]any{"maxOutputTokens": 50},
		"tools":            []any{map[string]any{"functionDeclarations": []any{map[string]any{"name": "f", "parameters": map[string]any{"type": "OBJECT", "properties": map[string]any{"x": map[string]any{"type": "STRING"}}}}}}}}
	rec := post(t, r, "/v1beta/models/auto:generateContent", r.APIKey(), in)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "Hi from upstream") || !strings.Contains(rec.Body.String(), `"finishReason":"STOP"`) {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if got["max_tokens"] != float64(50) || len(got["messages"].([]any)) != 2 {
		t.Fatalf("upstream: %v", got)
	}
	params := got["tools"].([]any)[0].(map[string]any)["function"].(map[string]any)["parameters"].(map[string]any)
	if params["type"] != "object" || params["properties"].(map[string]any)["x"].(map[string]any)["type"] != "string" {
		t.Fatalf("schema types should be lower-cased: %v", params)
	}
	rec = post(t, r, "/v1beta/models/auto:streamGenerateContent?alt=sse", r.APIKey(), in)
	if !strings.Contains(rec.Body.String(), "data: {") || !strings.Contains(rec.Body.String(), "usageMetadata") {
		t.Fatalf("stream: %s", rec.Body)
	}
}

func TestCompression(t *testing.T) {
	code := "func main() {\n\tif x {   \n\t\treturn\n\t}\n}\n\n\n\n\nend"
	body := map[string]any{"messages": []any{
		map[string]any{"role": "user", "content": code},
		map[string]any{"role": "tool", "content": strings.Repeat("same line\n", 50) + "\x1b[31mred\x1b[0m" + strings.Repeat("x", 20000)},
	}}
	if Compress(body, CompressOff) != 0 {
		t.Fatal("off changes nothing")
	}
	saved := Compress(body, CompressStandard)
	got := body["messages"].([]any)[0].(map[string]any)["content"].(string)
	if got != "func main() {\n\tif x {\n\t\treturn\n\t}\n}\n\nend" {
		t.Fatalf("light must keep indentation: %q", got)
	}
	tool := body["messages"].([]any)[1].(map[string]any)["content"].(string)
	if strings.Contains(tool, "\x1b[") || !strings.Contains(tool, "repeated 49 more times") || !strings.Contains(tool, "left out") || len(tool) > 9000 {
		t.Fatalf("tool output not trimmed (%d chars)", len(tool))
	}
	if saved < 10000 {
		t.Fatalf("saved %d", saved)
	}
	// Multi-byte text is never cut inside a character.
	s := cut(strings.Repeat("é", 3000), 1001, 1001, "x")
	if !strings.Contains(s, "left out") || strings.ContainsRune(s, '\uFFFD') {
		t.Fatal("bad cut")
	}
}

func TestMigrationKeepsChainAndKeys(t *testing.T) {
	home := t.TempDir()
	t.Setenv("ARGUS_HOME", home)
	keystore.Keychain = &memKC{m: map[string]string{}}
	cfg := config.Load()
	cfg.Chain = []config.Entry{{Provider: "groq", Model: "my-custom-model"}, {Provider: "gemini"}}
	cfg.CustomBase = "http://localhost:9999/v1"
	_ = cfg.Save()
	if _, err := cfg.SetKey("groq", "gsk_abc"); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(home + "/config.json")
	r := New(Options{Config: func() *config.Config { return cfg }})
	c := r.Settings().combo("my-chain")
	if c == nil || len(c.Targets) != 2 || c.Targets[0].Model != "my-custom-model" || c.Targets[1].Model == "" {
		t.Fatalf("chain should become a combo: %+v", c)
	}
	after, _ := os.ReadFile(home + "/config.json")
	if string(before) != string(after) {
		t.Fatal("migration must not rewrite config.json (shared with Argus CLI)")
	}
	if cfg.Key("groq") != "gsk_abc" {
		t.Fatal("keys stay in the shared store")
	}
	// Runs once.
	_ = r.DeleteCombo("my-chain")
	r2 := New(Options{Config: func() *config.Config { return cfg }})
	if r2.Settings().combo("my-chain") != nil {
		t.Fatal("migration should only run once")
	}
	// The custom provider is usable through the router with its saved address.
	if p, _ := config.ProviderByID(config.Custom); p.BaseURL != "http://localhost:9999/v1" {
		t.Fatalf("custom base %q", p.BaseURL)
	}
}

func TestServerStartsAndFallsBackToNextPort(t *testing.T) {
	r, _ := newTestRouter(t)
	r.mu.Lock()
	r.set.Port = 39000 + int(time.Now().UnixNano()%500)
	want := r.set.Port
	r.mu.Unlock()
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", want))
	if err == nil {
		defer ln.Close()
	}
	if err := r.Start(); err != nil {
		t.Fatal(err)
	}
	defer r.Stop()
	if r.Status().Port == want && err == nil {
		t.Fatal("the busy port should be skipped")
	}
	p, _ := config.ProviderByID(config.RouterID)
	if !strings.HasPrefix(p.BaseURL, r.URL()) {
		t.Fatalf("Argus's ArgusRouter provider should point at the server: %q vs %q", p.BaseURL, r.URL())
	}
	resp, err := http.Get(r.URL() + "/health")
	if err != nil || resp.StatusCode != 200 {
		t.Fatal("health", err)
	}
	resp.Body.Close()
	if r.Status().Level != "amber" {
		t.Fatalf("no keys: amber, got %+v", r.Status())
	}
	r.Stop()
	if p, _ := config.ProviderByID(config.RouterID); p.BaseURL != "" || r.Status().Level != "red" {
		t.Fatal("stopped router must not be usable")
	}
}

func TestPlanModels(t *testing.T) {
	h := func(w http.ResponseWriter, r *http.Request) { okJSON(w, "x") }
	a, _ := fakeProvider(t, h)
	r, _ := newTestRouter(t, a)
	if _, err := r.plan("nope-model", false); !errors.Is(err, ErrNoModel) {
		t.Fatal("unknown model should be refused")
	}
	pl, err := r.plan(a+"/other-model", false)
	if err != nil || pl.Targets[0].Model != "other-model" {
		t.Fatal("provider/model", pl, err)
	}
	pl, err = r.plan("m1", false)
	if err != nil || pl.Targets[0].Provider != a {
		t.Fatal("bare model id should find its provider", pl, err)
	}
	for _, m := range []string{"", "auto", "argusrouter/auto"} {
		if pl, err := r.plan(m, false); err != nil || pl.Combo != "auto" {
			t.Fatal(m, err)
		}
	}
	if pl, err := r.plan("auto/fast", false); err != nil || pl.Combo != "auto/fast" || len(pl.Targets) != 1 {
		t.Fatal("auto/fast", pl, err)
	}
	found := false
	for _, m := range r.ModelList() {
		found = found || m.ID == a+"/m1"
	}
	if !found {
		t.Fatal("models list should include provider/model")
	}
	// ArgusRouter never routes to itself.
	cfg := r.cfg()
	cfg.Chain = append(cfg.Chain, config.Entry{Provider: config.RouterID, Model: "auto"})
	for _, tg := range r.autoTargets(cfg) {
		if tg.Provider == config.RouterID {
			t.Fatal("loop")
		}
	}
}
