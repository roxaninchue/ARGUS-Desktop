package cloudai

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/lacksonkapapahue/blaze-desktop/internal/llm"
)

// Every step of one turn carries the same X-Blaze-Turn; X-Blaze-Prefer appears from step 2 on
// with the route that answered step 1, and stays on it even when a later step is served by
// another route. A new turn gets a new id and no pin.
func TestTurnHeadersAcrossSteps(t *testing.T) {
	var mu sync.Mutex
	var seen []http.Header
	n := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = append(seen, r.Header.Clone())
		n++
		step := n
		mu.Unlock()
		model := "llama-a"
		if step == 3 {
			model = "qwen-b" // the gateway fell back on step 3
		}
		w.Header().Set("X-Blaze-Provider", "groq")
		w.Header().Set("X-Blaze-Model", model)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`)
	}))
	defer srv.Close()
	g := NewGateway(srv.URL+"/v1", func() string { return "bz_test_token_aaaaaaaaaaaaaaaaaaaaa" })
	msgs := []llm.Message{{Role: "user", Content: "hi"}}

	turn := NewTurn()
	ctx := WithTurn(context.Background(), turn)
	for i := 0; i < 4; i++ {
		if _, err := g.Complete(ctx, msgs, nil, nil, false); err != nil {
			t.Fatal(err)
		}
	}
	if seen[0].Get(TurnHeader) != turn.ID || seen[0].Get(PreferHeader) != "" {
		t.Fatalf("step 1 headers: %v", seen[0])
	}
	for i := 1; i < 4; i++ {
		if seen[i].Get(TurnHeader) != turn.ID {
			t.Fatalf("step %d turn id %q, want %q", i+1, seen[i].Get(TurnHeader), turn.ID)
		}
		if seen[i].Get(PreferHeader) != "groq/llama-a" {
			t.Fatalf("step %d prefer %q", i+1, seen[i].Get(PreferHeader))
		}
	}
	if turn.Switches() != 2 { // a → b (step 3) → a (step 4)
		t.Fatalf("switches = %d", turn.Switches())
	}

	// No turn on the context (background calls, Blaze CLI): no headers.
	if _, err := g.Complete(context.Background(), msgs, nil, nil, true); err != nil {
		t.Fatal(err)
	}
	if h := seen[4]; h.Get(TurnHeader) != "" || h.Get(PreferHeader) != "" {
		t.Fatalf("headers without a turn: %v", h)
	}
	// A new turn starts unpinned.
	t2 := NewTurn()
	if t2.ID == turn.ID || t2.Pin() != "" {
		t.Fatal("new turn should have a new id and no pin")
	}
}

func TestHeaderSafe(t *testing.T) {
	if got := headerSafe("groq/llama 3\r\nX-Evil: 1"); strings.ContainsAny(got, " \r\n") {
		t.Fatalf("unsafe header value %q", got)
	}
	if len(headerSafe(strings.Repeat("a", 500))) != 120 {
		t.Fatal("header value not capped at 120")
	}
	var nilTurn *Turn
	nilTurn.Served("x") // no panic
	nilTurn.Apply(http.Header{})
}

// Prewarm: signed in it POSTs /v1/warm with the account token (gateway 0.6.9); signed out it hits
// the root's /health without a token. At most once per PrewarmEvery, and the connection stays in
// the pool for the chat request.
func TestPrewarm(t *testing.T) {
	var health, warm, chat, conns atomic.Int32
	var authOnHealth, authOnWarm atomic.Value
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			health.Add(1)
			authOnHealth.Store(r.Header.Get("Authorization"))
			fmt.Fprint(w, `{"ok":true,"service":"blaze-gateway","version":"0.6.1"}`)
		case "/v1/warm":
			if r.Method == "POST" {
				warm.Add(1)
				authOnWarm.Store(r.Header.Get("Authorization"))
			}
			fmt.Fprint(w, `{"ok":true,"version":"0.6.9","user":true,"ms":3}`)
		case "/v1/chat/completions":
			chat.Add(1)
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	srv.Config.ConnState = func(_ net.Conn, s http.ConnState) {
		if s == http.StateNew {
			conns.Add(1)
		}
	}
	srv.Start()
	defer srv.Close()
	if HealthURL(srv.URL+"/v1") != srv.URL+"/health" || HealthURL("https://x.dev/v1/") != "https://x.dev/health" {
		t.Fatal("HealthURL")
	}
	g := NewGateway(srv.URL+"/v1", func() string { return "bz_test_token_aaaaaaaaaaaaaaaaaaaaa" })
	if !g.Prewarm(context.Background()) {
		t.Fatal("first prewarm should run")
	}
	if g.Prewarm(context.Background()) {
		t.Fatal("second prewarm within a minute should be skipped")
	}
	if warm.Load() != 1 || health.Load() != 0 {
		t.Fatalf("signed in: warm hits = %d, health hits = %d", warm.Load(), health.Load())
	}
	if a, _ := authOnWarm.Load().(string); a != "Bearer bz_test_token_aaaaaaaaaaaaaaaaaaaaa" {
		t.Fatalf("warm must carry the token, got %q", a)
	}
	if WarmURL(srv.URL+"/v1") != srv.URL+"/v1/warm" {
		t.Fatal("WarmURL")
	}
	if _, err := g.Complete(context.Background(), []llm.Message{{Role: "user", Content: "hi"}}, nil, nil, false); err != nil {
		t.Fatal(err)
	}
	if chat.Load() != 1 || conns.Load() != 1 {
		t.Fatalf("the chat request should reuse the warmed connection: chats=%d conns=%d", chat.Load(), conns.Load())
	}
	out := NewGateway(srv.URL+"/v1", func() string { return "" })
	if !out.Prewarm(context.Background()) || health.Load() != 1 {
		t.Fatal("signed out: prewarm should hit /health")
	}
	if a, _ := authOnHealth.Load().(string); a != "" {
		t.Fatal("/health must not get a token")
	}
}
