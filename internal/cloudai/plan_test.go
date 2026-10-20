package cloudai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPlan(t *testing.T) {
	var auth, path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth, path = r.Header.Get("Authorization"), r.URL.Path
		w.Write([]byte(`{"plan":"basic","term":{"plan":"Pro"}}`))
	}))
	defer srv.Close()
	g := &Gateway{Base: srv.URL + "/v1", Token: func() string { return "tok" }, HC: srv.Client()}
	p, err := g.Plan(context.Background())
	if err != nil || p != "pro" || auth != "Bearer tok" || path != "/v1/billing/me" {
		t.Fatalf("%q %v %q %q", p, err, auth, path)
	}
	g.Token = func() string { return "" }
	if _, err := g.Plan(context.Background()); err == nil {
		t.Fatal("signed out must not ask")
	}
}
