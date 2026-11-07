package cloudai

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lacksonkapapahue/argus-desktop/internal/llm"
)

func TestUsageMeter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/usage":
			if r.Header.Get("Authorization") != "Bearer tok" {
				w.WriteHeader(401)
				return
			}
			fmt.Fprint(w, `{"plan":"basic","day":"2026-10-20","resets_at":"2026-10-21T00:00:00Z","requests":{"used":12,"limit":150,"remaining":138},"tokens":{"used":9120,"limit":null,"remaining":null},"last_reply":{"tokens":710}}`)
		case "/v1/chat/completions":
			h := w.Header()
			h.Set("Content-Type", "application/json")
			h.Set("X-Blaze-Plan", "basic")
			h.Set("X-Blaze-Requests-Used", "13")
			h.Set("X-Blaze-Requests-Limit", "150")
			h.Set("X-RateLimit-Remaining-Requests", "137")
			h.Set("X-Argus-Tokens-Used", "9120")
			h.Set("X-Argus-Tokens-Limit", "300000")
			h.Set("X-Argus-Tokens-Remaining", "290880")
			h.Set("X-Blaze-Quota-Reset", "2026-10-21T00:00:00Z")
			h.Set("X-Blaze-Fast-Lane", "1")
			fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"ok"}}],"usage":{"prompt_tokens":600,"completion_tokens":100,"total_tokens":700}}`)
		}
	}))
	defer srv.Close()
	g := NewGateway(srv.URL+"/v1", func() string { return "tok" })
	q, err := g.Usage(context.Background())
	if err != nil || q.RequestsLeft != 138 || q.TokensLimit != -1 || q.TokensLeft != -1 || q.LastTokens != 710 || q.Plan != "basic" {
		t.Fatalf("usage: %v %+v", err, q)
	}
	if _, err := g.Complete(context.Background(), []llm.Message{{Role: "user", Content: "hi"}}, nil, nil, false); err != nil {
		t.Fatal(err)
	}
	q = g.LastQuota()
	if q == nil || q.RequestsUsed != 13 || q.RequestsLeft != 137 || q.TokensLimit != 300000 || q.TokensUsed != 9820 || q.TokensLeft != 290180 || q.LastTokens != 700 {
		t.Fatalf("after a reply: %+v", q)
	}
	g.Token = func() string { return "" }
	if _, err := g.Usage(context.Background()); err != ErrSignedOut {
		t.Fatalf("signed out: %v", err)
	}
}
