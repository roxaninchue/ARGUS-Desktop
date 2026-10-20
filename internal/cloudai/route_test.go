package cloudai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lacksonkapapahue/blaze-desktop/internal/config"
	"github.com/lacksonkapapahue/blaze-desktop/internal/llm"
)

const goodToken = "bz_abcdefghijklmnopqrstuvwxyz0123"

// fakeGateway is the Blaze gateway's API surface: POST /v1/chat/completions, GET /v1/models.
type fakeGateway struct {
	mu     sync.Mutex
	bodies []map[string]any
	auth   []string
	reply  func(n int, w http.ResponseWriter, body map[string]any)
}

func (f *fakeGateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == "/v1/models" && r.Method == "GET":
		if r.Header.Get("Authorization") != "Bearer "+goodToken {
			w.WriteHeader(403)
			io.WriteString(w, `{"error":{"message":"You're on the Blaze waiting list.","type":"permission_error","code":"waitlisted"}}`)
			return
		}
		io.WriteString(w, `{"object":"list","data":[{"id":"blaze-auto"}]}`)
	case r.URL.Path == "/v1/chat/completions" && r.Method == "POST":
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		n := len(f.bodies)
		f.bodies = append(f.bodies, body)
		f.auth = append(f.auth, r.Header.Get("Authorization"))
		f.mu.Unlock()
		f.reply(n, w, body)
	default:
		http.NotFound(w, r)
	}
}

func sse(w http.ResponseWriter, prov, model string, parts ...string) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("X-Blaze-Provider", prov)
	w.Header().Set("X-Blaze-Model", model)
	for _, p := range parts {
		b, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"delta": map[string]any{"content": p}}}})
		fmt.Fprintf(w, "data: %s\n\n", b)
	}
	io.WriteString(w, "data: [DONE]\n\n")
}

func noSleep(context.Context, time.Duration) error { return nil }

func newGW(t *testing.T, f *fakeGateway, token string) (*Gateway, func()) {
	srv := httptest.NewServer(f)
	g := NewGateway(BaseFrom(srv.URL), func() string { return token })
	g.Sleep = noSleep
	return g, srv.Close
}

func TestBaseFrom(t *testing.T) {
	t.Setenv("BLAZE_GATEWAY_URL", "")
	if BaseFrom("") != DefaultBase || BaseFrom("https://x.dev/") != "https://x.dev/v1" || BaseFrom("https://x.dev/v1") != "https://x.dev/v1" {
		t.Error("base")
	}
	t.Setenv("BLAZE_GATEWAY_URL", "http://127.0.0.1:9")
	if BaseFrom("https://x.dev") != "http://127.0.0.1:9/v1" {
		t.Error("env wins")
	}
}

func TestGatewayRequestShape(t *testing.T) {
	f := &fakeGateway{reply: func(n int, w http.ResponseWriter, b map[string]any) { sse(w, "groq", "llama-3.3", "Hel", "lo") }}
	g, stop := newGW(t, f, goodToken)
	defer stop()
	var got strings.Builder
	r, err := g.Complete(context.Background(), user("hey"), nil, func(s string) { got.WriteString(s) }, false)
	if err != nil {
		t.Fatal(err)
	}
	if r.Text != "Hello" || got.String() != "Hello" || r.Route != "Blaze Cloud" || r.Model != Model || r.Provider != ProviderID {
		t.Fatalf("result %+v / %q", r, got.String())
	}
	b := f.bodies[0]
	if b["model"] != "blaze-auto" || b["blaze_task"] != "quick" || b["stream"] != true || f.auth[0] != "Bearer "+goodToken {
		t.Fatalf("body %v auth %v", b, f.auth)
	}
	if _, ok := b["blaze_avoid"]; ok {
		t.Error("no avoid on a first try")
	}
	_, _ = g.Complete(context.Background(), user("open my notes"), tool("read_file"), nil, false)
	if f.bodies[1]["blaze_task"] != "agent" || f.bodies[1]["tool_choice"] != "auto" || f.bodies[1]["stream"] != nil {
		t.Errorf("agent body %v", f.bodies[1])
	}
}

func TestGatewaySignedOut(t *testing.T) {
	f := &fakeGateway{reply: func(int, http.ResponseWriter, map[string]any) { t.Error("no call when signed out") }}
	g, stop := newGW(t, f, "")
	defer stop()
	_, err := g.Complete(context.Background(), user("hi"), nil, nil, false)
	if err == nil || err.Error() != SignedOut || llm.Plain(err) != SignedOut {
		t.Fatalf("signed out: %v", err)
	}
	if s := g.Check(context.Background()); s.Ready || s.Line != SignedOut {
		t.Errorf("check signed out: %+v", s)
	}
}

func TestGatewayErrorsAndRetries(t *testing.T) {
	f := &fakeGateway{reply: func(n int, w http.ResponseWriter, b map[string]any) {
		if n < 2 {
			w.Header().Set("Retry-After", "2")
			w.WriteHeader(503)
			io.WriteString(w, `{"error":{"message":"Every model is cooling","code":"blaze_models_busy"}}`)
			return
		}
		sse(w, "cf", "llama", "ok")
	}}
	g, stop := newGW(t, f, goodToken)
	var waits []time.Duration
	g.Sleep = func(_ context.Context, d time.Duration) error { waits = append(waits, d); return nil }
	r, err := g.Complete(context.Background(), user("hey"), nil, func(string) {}, false)
	stop()
	if err != nil || r.Text != "ok" || len(f.bodies) != 3 || len(waits) != 2 || waits[0] != 2*time.Second {
		t.Fatalf("retry: %v %v calls=%d waits=%v", r, err, len(f.bodies), waits)
	}

	for _, c := range []struct {
		status int
		body   string
		want   string
		calls  int
	}{
		{401, `{"error":{"message":"Your Blaze session has expired.","code":"invalid_api_key"}}`, Expired, 1},
		{403, `{"error":{"message":"You're on the Blaze waiting list.","code":"waitlisted"}}`, Waitlisted, 1},
		{403, `{"error":{"message":"no","code":"access_rejected"}}`, Rejected, 1},
		{429, `{"error":{"message":"Daily allowance used","code":"quota_exceeded"}}`, Allowance, 1},
		{500, `{"error":{"message":"Something went wrong in the gateway","code":"internal_error"}}`, Busy, 3},
	} {
		f := &fakeGateway{reply: func(n int, w http.ResponseWriter, b map[string]any) {
			w.WriteHeader(c.status)
			io.WriteString(w, c.body)
		}}
		g, stop := newGW(t, f, goodToken)
		_, err := g.Complete(context.Background(), user("hi"), nil, nil, false)
		stop()
		if err == nil || err.Error() != c.want || len(f.bodies) != c.calls {
			t.Errorf("%d: err=%v calls=%d", c.status, err, len(f.bodies))
		}
	}

	g2 := NewGateway("http://127.0.0.1:1/v1", func() string { return goodToken })
	g2.Sleep = noSleep
	if _, err := g2.Complete(context.Background(), user("hi"), nil, nil, false); err == nil || err.Error() != Unreachable {
		t.Errorf("unreachable: %v", err)
	}
}

func TestGatewayEmptyThenAvoid(t *testing.T) {
	f := &fakeGateway{reply: func(n int, w http.ResponseWriter, b map[string]any) {
		switch n {
		case 0:
			sse(w, "groq", "qwen", "<think>only thinking</think>")
		case 1:
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Blaze-Provider", "groq")
			w.Header().Set("X-Blaze-Model", "qwen")
			io.WriteString(w, `{"choices":[{"message":{"content":""}}]}`)
		default:
			sse(w, "cf", "llama", "Here you go")
		}
	}}
	g, stop := newGW(t, f, goodToken)
	defer stop()
	r, err := g.Complete(context.Background(), user("hey"), nil, func(string) {}, false)
	if err != nil || r.Text != "Here you go" {
		t.Fatalf("%v %v", r, err)
	}
	if f.bodies[1]["stream"] != nil {
		t.Error("second try is not streamed")
	}
	msgs := f.bodies[1]["messages"].([]any)
	if last := msgs[len(msgs)-1].(map[string]any); last["content"] != Nudge {
		t.Error("second try carries the nudge")
	}
	av, _ := f.bodies[2]["blaze_avoid"].([]any)
	if len(av) != 1 || av[0] != "groq/qwen" {
		t.Errorf("third try avoids the empty route: %v", f.bodies[2]["blaze_avoid"])
	}
}

func TestCheckWaitlisted(t *testing.T) {
	f := &fakeGateway{}
	g, stop := newGW(t, f, "bz_someone_else_waiting_1234567")
	defer stop()
	if s := g.Check(context.Background()); s.Ready || s.Line != Waitlisted {
		t.Errorf("waitlisted: %+v", s)
	}
	g.Token = func() string { return goodToken }
	if s := g.Check(context.Background()); !s.Ready {
		t.Errorf("ready: %+v", s)
	}
}

func TestChoiceLoadSave(t *testing.T) {
	dir := t.TempDir()
	if c := LoadChoice(dir, false); !c.Cloud() {
		t.Error("new users start on Blaze Cloud")
	}
	if c := LoadChoice(dir, true); c.Cloud() {
		t.Error("a person who picked their own model keeps Custom")
	}
	c := LoadChoice(dir, false)
	c.Mode = ModeCustom
	c.FallbackToCloud = true
	if !c.SetOverride("code", "groq", "qwen-coder") || !c.SetOverride("vision", "gemini", "gemini-3.8-flash") || c.SetOverride("long", "x", "y") {
		t.Fatal("overrides")
	}
	_ = os.MkdirAll(filepath.Join(dir, "desktop"), 0o700)
	_ = os.WriteFile(Path(dir), []byte(`{"mode":"cloud","future_key":42}`), 0o600)
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(Path(dir))
	if !strings.Contains(string(raw), `"future_key": 42`) {
		t.Errorf("unknown key dropped: %s", raw)
	}
	if st, _ := os.Stat(Path(dir)); st.Mode().Perm() != 0o600 {
		t.Errorf("mode %v", st.Mode().Perm())
	}
	d := LoadChoice(dir, false)
	if d.Cloud() || !d.FallbackToCloud || d.Overrides["code"].Model != "qwen-coder" || len(d.Overrides) != 2 {
		t.Fatalf("reload %+v", d)
	}
	d.SetOverride("code", "", "")
	_ = d.Save()
	if e := LoadChoice(dir, false); len(e.Overrides) != 1 {
		t.Error("clearing an override")
	}
	_ = os.WriteFile(Path(dir), []byte(`{"mode":"weird","overrides":{"chat":{"provider":"","model":"m"}}}`), 0o600)
	if e := LoadChoice(dir, true); !e.Cloud() || len(e.Overrides) != 0 {
		t.Errorf("normalize %+v", e)
	}
}

func TestCustomTask(t *testing.T) {
	img := pngFile(t)
	if CustomTask(user("hello")) != "chat" || CustomTask(user("debug this python")) != "code" {
		t.Error("chat/code")
	}
	if CustomTask([]llm.Message{{Role: "user", Content: "hi", Attach: []llm.Attach{{Kind: "image", Path: img}}}}) != "vision" {
		t.Error("vision")
	}
}

// fakeProvider is a user's own OpenAI-compatible service (the Custom provider).
func fakeProvider(status int, text string, seen *[]string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var b map[string]any
		_ = json.NewDecoder(r.Body).Decode(&b)
		*seen = append(*seen, fmt.Sprint(b["model"]))
		if status != 200 {
			w.WriteHeader(status)
			io.WriteString(w, `{"error":{"message":"invalid api key"}}`)
			return
		}
		sse(w, "", "", text)
	}))
}

func customCfg(t *testing.T, base string) *config.Config {
	t.Setenv("BLAZE_HOME", t.TempDir())
	cfg := config.Load()
	cfg.CustomBase = base
	cfg.Chain = []config.Entry{{Provider: config.Custom, Model: "my-model"}}
	cfg.Default = &config.Entry{Provider: config.Custom, Model: "my-model"}
	cfg.NoFallback = true
	cfg.NoKeyless = true
	_ = cfg.Save()
	return config.Load()
}

func TestRoutingCloudVsCustom(t *testing.T) {
	gwf := &fakeGateway{reply: func(n int, w http.ResponseWriter, b map[string]any) { sse(w, "groq", "llama", "from cloud") }}
	gw, stop := newGW(t, gwf, goodToken)
	defer stop()
	var provSeen []string
	prov := fakeProvider(200, "from custom", &provSeen)
	defer prov.Close()
	cfg := customCfg(t, prov.URL+"/v1")
	dir := t.TempDir()

	rt := NewRouter(LoadChoice(dir, false), gw)
	llm.SetBackend(rt)
	defer llm.SetBackend(nil)

	r, err := llm.CompleteWith(context.Background(), cfg, user("hello"), nil, llm.Opts{})
	if err != nil || r.Text != "from cloud" || len(provSeen) != 0 || len(gwf.bodies) != 1 {
		t.Fatalf("cloud: %v %v prov=%v", r, err, provSeen)
	}
	if q, err := llm.Quick(context.Background(), cfg, user("title this"), 5*time.Second); err != nil || q != "from cloud" || gwf.bodies[1]["blaze_task"] != "quick" {
		t.Fatalf("cloud quick: %q %v %v", q, err, gwf.bodies[1])
	}

	_ = rt.Update(func(c *Choice) { c.Mode = ModeCustom })
	r, err = llm.CompleteWith(context.Background(), cfg, user("hello"), nil, llm.Opts{})
	if err != nil || r.Text != "from custom" || len(gwf.bodies) != 2 || provSeen[0] != "my-model" {
		t.Fatalf("custom: %v %v gw=%d prov=%v", r, err, len(gwf.bodies), provSeen)
	}

	_ = rt.Update(func(c *Choice) { c.SetOverride("code", config.Custom, "coder-model") })
	if _, err = llm.CompleteWith(context.Background(), cfg, user("refactor this golang function"), nil, llm.Opts{}); err != nil || provSeen[len(provSeen)-1] != "coder-model" {
		t.Fatalf("pin: %v %v", err, provSeen)
	}
	if _, err = llm.CompleteWith(context.Background(), cfg, user("hello"), nil, llm.Opts{}); err != nil || provSeen[len(provSeen)-1] != "my-model" {
		t.Fatalf("unpinned chat: %v %v", err, provSeen)
	}
	if LoadChoice(dir, false).Overrides["code"].Model != "coder-model" {
		t.Error("choice saved")
	}
}

func TestCustomFailureNoSilentSwitch(t *testing.T) {
	gwf := &fakeGateway{reply: func(n int, w http.ResponseWriter, b map[string]any) { sse(w, "groq", "llama", "from cloud") }}
	gw, stop := newGW(t, gwf, goodToken)
	defer stop()
	var provSeen []string
	prov := fakeProvider(401, "", &provSeen)
	defer prov.Close()
	cfg := customCfg(t, prov.URL+"/v1")
	rt := NewRouter(LoadChoice(t.TempDir(), true), gw)

	_, err, ok := rt.Complete(context.Background(), cfg, user("hello"), nil, llm.Opts{})
	if !ok || err == nil || len(gwf.bodies) != 0 {
		t.Fatalf("no silent switch: %v gw=%d", err, len(gwf.bodies))
	}
	line := llm.Plain(err)
	if !strings.HasPrefix(line, "Your custom model couldn't answer") || !strings.Contains(line, "Blaze Cloud wasn't asked") {
		t.Errorf("line: %q", line)
	}
	var re *llm.RouteError
	if !errors.As(err, &re) {
		t.Error("the router's failures stay reachable for Details")
	}

	_ = rt.Update(func(c *Choice) { c.FallbackToCloud = true })
	var status []string
	r, err, _ := rt.Complete(context.Background(), cfg, user("hello"), nil, llm.Opts{OnStatus: func(s string) { status = append(status, s) }})
	if err != nil || r.Text != "from cloud" || r.Note != "Your custom model couldn't answer, so Blaze Cloud did." || len(status) != 1 {
		t.Fatalf("fallback: %v %v %v", r, err, status)
	}
}
