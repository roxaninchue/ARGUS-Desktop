package main

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/zalando/go-keyring"

	"github.com/lacksonkapapahue/blaze-desktop/internal/cloud"
	"github.com/lacksonkapapahue/blaze-desktop/internal/connect"
)

// v0.19.13: the Blaze account session and one-click refresh against a fake blaze-auth.
// A 401 signs out cleanly; offline, timeouts, 5xx and garbled answers keep the session.

func accountApp(t *testing.T, base string) *App {
	t.Helper()
	keyring.MockInit()
	t.Setenv("BLAZE_HOME", t.TempDir())
	c := cloud.New(base)
	c.HC.Timeout = 2 * time.Second
	a := &App{cloud: &cloudState{client: c, sec: connect.Keychain()}}
	if err := a.cloud.sec.Set(sessionKey, "sess-1"); err != nil {
		t.Fatal(err)
	}
	u := &cloud.User{ID: "u1", Email: "a@example.com"}
	a.cloud.user = u
	a.saveUser(u)
	return a
}

func signedIn(a *App) bool {
	tok, _ := a.cloud.sec.Get(sessionKey)
	_, err := os.Stat(accountFile())
	return tok == "sess-1" && a.cloud.user != nil && err == nil
}

func TestCheckSessionUnauthorizedSignsOut(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		io.WriteString(w, `{"error":"unauthorized"}`)
	}))
	defer srv.Close()
	a := accountApp(t, srv.URL)
	a.checkSession() // must not panic with no Wails context
	if tok, _ := a.cloud.sec.Get(sessionKey); tok != "" {
		t.Fatal("session token should be gone after a 401")
	}
	if a.cloud.user != nil {
		t.Fatal("user should be cleared")
	}
	if _, err := os.Stat(accountFile()); !os.IsNotExist(err) {
		t.Fatal("cached account file should be removed")
	}
}

// dnsFail is a transport whose every lookup fails like an offline resolver (no proxy:
// the sandbox's HTTP proxy would otherwise answer for any host).
func dnsFail() *http.Transport {
	return &http.Transport{DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
		return nil, &net.DNSError{Err: "no such host", Name: addr, IsNotFound: true}
	}}
}

func TestCheckSessionPassingFailuresKeepSession(t *testing.T) {
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	refused := "http://" + ln.Addr().String()
	ln.Close()
	block := make(chan struct{})
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-block:
		case <-r.Context().Done():
		}
	}))
	defer slow.Close()
	defer close(block)
	html502 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(502)
		io.WriteString(w, "<html>Bad gateway</html>")
	}))
	defer html502.Close()
	garbled := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"user":`)
	}))
	defer garbled.Close()
	rate := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(429)
		io.WriteString(w, `{"error":"rate_limited"}`)
	}))
	defer rate.Close()

	for name, base := range map[string]string{
		"refused": refused, "dns": "http://blaze-auth.does-not-exist.invalid", "timeout": slow.URL,
		"502 html": html502.URL, "garbled": garbled.URL, "429": rate.URL,
	} {
		a := accountApp(t, base)
		if name == "timeout" {
			a.cloud.client.HC.Timeout = 300 * time.Millisecond
		}
		if name == "dns" {
			a.cloud.client.HC.Transport = dnsFail()
		}
		a.checkSession()
		if !signedIn(a) {
			t.Fatalf("%s: a passing failure must not sign the user out", name)
		}
	}
}

func TestCheckSessionRefreshesUser(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer sess-1" {
			w.WriteHeader(401)
			return
		}
		io.WriteString(w, `{"user":{"id":"u1","email":"new@example.com","name":"Ana"}}`)
	}))
	defer srv.Close()
	a := accountApp(t, srv.URL)
	a.checkSession()
	if a.cloud.user == nil || a.cloud.user.Email != "new@example.com" {
		t.Fatalf("user not refreshed: %+v", a.cloud.user)
	}
}

func TestBrokerRefreshClassifiesErrors(t *testing.T) {
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	refused := "http://" + ln.Addr().String()
	ln.Close()
	answer := func(status int, body string) string {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
			io.WriteString(w, body)
		}))
		t.Cleanup(srv.Close)
		return srv.URL
	}
	for name, tc := range map[string]struct {
		base string
		want error // nil: a real refusal (neither sentinel)
	}{
		"refused":       {refused, connect.ErrBrokerUnreachable},
		"503 html":      {answer(503, "<html>down</html>"), connect.ErrBrokerTemporary},
		"500 json":      {answer(500, `{"error":"internal"}`), connect.ErrBrokerTemporary},
		"429":           {answer(429, `{"error":"rate_limited"}`), connect.ErrBrokerTemporary},
		"200 garbled":   {answer(200, `<html>portal</html>`), connect.ErrBrokerTemporary},
		"200 no token":  {answer(200, `{"kind":"tokens","tokens":{}}`), connect.ErrBrokerTemporary},
		"invalid_grant": {answer(400, `{"error":"invalid_grant"}`), nil},
		"401":           {answer(401, `{"error":"unauthorized"}`), nil},
	} {
		a := accountApp(t, tc.base)
		b := &broker{cs: a.cloud}
		tok, err := b.Refresh(context.Background(), "google", "rt")
		if tok != nil || err == nil {
			t.Fatalf("%s: want an error, got %+v", name, tok)
		}
		isU, isT := errors.Is(err, connect.ErrBrokerUnreachable), errors.Is(err, connect.ErrBrokerTemporary)
		switch tc.want {
		case nil:
			if isU || isT {
				t.Fatalf("%s: a refusal must not look temporary: %v", name, err)
			}
		default:
			if !errors.Is(err, tc.want) {
				t.Fatalf("%s: want %v, got %v", name, tc.want, err)
			}
		}
	}

	ok := answer(200, `{"kind":"tokens","provider":"google","tokens":{"access_token":"at","refresh_token":"rt2","expires_in":3600}}`)
	a := accountApp(t, ok)
	tok, err := (&broker{cs: a.cloud}).Refresh(context.Background(), "google", "rt")
	if err != nil || tok.AccessToken != "at" || tok.RefreshToken != "rt2" || tok.ExpiresAt <= time.Now().UnixMilli() {
		t.Fatalf("success: %+v %v", tok, err)
	}
}
