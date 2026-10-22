package cloud

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// v0.19.13: error paths of the blaze-auth client. Every failure must come back as a
// typed error with a plain sentence, never a panic or a raw parser message.

func serve(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return New(srv.URL)
}

// refusedURL is a local address nothing listens on (connection refused).
func refusedURL(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	u := "http://" + ln.Addr().String()
	ln.Close()
	return u
}

// dnsFail is a transport whose every lookup fails like an offline resolver (no proxy:
// the sandbox's HTTP proxy would otherwise answer for any host).
func dnsFail() *http.Transport {
	return &http.Transport{DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
		return nil, &net.DNSError{Err: "no such host", Name: addr, IsNotFound: true}
	}}
}

func TestErrOfflineIsUnreachable(t *testing.T) {
	for name, base := range map[string]string{
		"refused": refusedURL(t),
		"dns":     "http://blaze-auth.does-not-exist.invalid",
	} {
		c := New(base)
		c.HC.Timeout = 5 * time.Second
		if name == "dns" {
			c.HC.Transport = dnsFail()
		}
		_, err := c.Me(context.Background(), "tok")
		var un *Unreachable
		if !errors.As(err, &un) {
			t.Fatalf("%s: want Unreachable, got %T %v", name, err, err)
		}
		if m := Message(err); !strings.Contains(m, "internet connection") {
			t.Fatalf("%s: message %q", name, m)
		}
		if _, err := c.Refresh(context.Background(), "google", "rt"); !errors.As(err, &un) {
			t.Fatalf("%s refresh: want Unreachable, got %v", name, err)
		}
	}
}

func TestErrTimeoutIsUnreachableNotCancelled(t *testing.T) {
	block := make(chan struct{})
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-block:
		case <-r.Context().Done():
		}
	})
	defer close(block)

	// The client's own timeout.
	c.HC.Timeout = 150 * time.Millisecond
	_, err := c.Me(context.Background(), "tok")
	var un *Unreachable
	if !errors.As(err, &un) {
		t.Fatalf("client timeout: want Unreachable, got %T %v", err, err)
	}

	// A context deadline (checkSession, refreshProviders) is a slow network, not "Sign-in was cancelled".
	c.HC.Timeout = 0
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	_, err = c.Me(ctx, "tok")
	if errors.Is(err, ErrCancelled) || !errors.As(err, &un) {
		t.Fatalf("ctx deadline: want Unreachable, got %T %v", err, err)
	}
	if strings.Contains(Message(err), "cancelled") {
		t.Fatalf("deadline must not read as cancelled: %q", Message(err))
	}

	// A real cancel is still ErrCancelled.
	ctx2, cancel2 := context.WithCancel(context.Background())
	go func() { time.Sleep(100 * time.Millisecond); cancel2() }()
	if _, err := c.Me(ctx2, "tok"); !errors.Is(err, ErrCancelled) {
		t.Fatalf("cancel: want ErrCancelled, got %v", err)
	}
}

func TestErr5xxHTMLBody(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(502)
		w.Write([]byte("<!doctype html><title>502 Bad Gateway</title><h1>Cloudflare</h1>"))
	})
	_, err := c.Me(context.Background(), "tok")
	var ae *APIError
	if !errors.As(err, &ae) || ae.Status != 502 || ae.Code != "http_502" {
		t.Fatalf("want APIError 502 http_502, got %T %v", err, err)
	}
	m := Message(err)
	if strings.Contains(m, "<") || !strings.Contains(m, "having trouble") {
		t.Fatalf("message %q", m)
	}
	if _, err := c.Refresh(context.Background(), "google", "rt"); !errors.As(err, &ae) || ae.Status != 502 {
		t.Fatalf("refresh: %v", err)
	}
}

func TestErr5xxJSONKeepsCode(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte(`{"error":"mail_failed"}`))
	})
	_, err := c.Me(context.Background(), "tok")
	if m := Message(err); m != "Blaze couldn't send the email. Try again in a moment." {
		t.Fatalf("message %q", m)
	}
}

func TestErrMalformedJSON(t *testing.T) {
	for name, body := range map[string]string{
		"html":      "<html>captive portal</html>",
		"truncated": `{"user":{"id":"u1","email":"a@b`,
		"wrongtype": `{"user":"nope"}`,
	} {
		b := body
		c := serve(t, func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(b)) })
		u, err := c.Me(context.Background(), "tok")
		if u != nil || !errors.Is(err, ErrBadAnswer) {
			t.Fatalf("%s: want ErrBadAnswer, got %v %v", name, u, err)
		}
		if m := Message(err); strings.Contains(m, "invalid character") || !strings.Contains(m, "couldn't read") {
			t.Fatalf("%s: message %q", name, m)
		}
	}
}

func TestErrRefreshNoToken(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"kind":"tokens","provider":"google","tokens":{"access_token":""}}`))
	})
	_, err := c.Refresh(context.Background(), "google", "rt")
	var ae *APIError
	if !errors.As(err, &ae) || ae.Code != "no_token" {
		t.Fatalf("want no_token, got %v", err)
	}
}

func TestErrUnauthorizedAndRateLimit(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/me" {
			if r.Header.Get("Authorization") != "Bearer tok" {
				t.Errorf("bearer: %q", r.Header.Get("Authorization"))
			}
			w.WriteHeader(401)
			w.Write([]byte(`{"error":"unauthorized"}`))
			return
		}
		w.WriteHeader(429)
		w.Write([]byte(`{"error":"rate_limited","retry_after":42}`))
	})
	_, err := c.Me(context.Background(), "tok")
	var ae *APIError
	if !errors.As(err, &ae) || ae.Status != 401 || Message(err) != "You're signed out. Sign in again." {
		t.Fatalf("401: %v / %q", err, Message(err))
	}
	_, err = c.Refresh(context.Background(), "google", "rt")
	if !errors.As(err, &ae) || ae.Status != 429 || ae.Retry != 42 || !strings.Contains(Message(err), "Too many tries") {
		t.Fatalf("429: %+v", ae)
	}
}

func TestErrRefreshNotConfigured(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(503)
		w.Write([]byte(`{"error":"not_configured"}`))
	})
	if _, err := c.Refresh(context.Background(), "spotify", "rt"); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("want ErrNotConfigured, got %v", err)
	}
}

func TestErrHugeBodyIsBounded(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte(`{"error":"` + strings.Repeat("x", 2<<20) + `"}`))
	})
	_, err := c.Me(context.Background(), "tok")
	var ae *APIError
	// The body is cut at 1 MiB, so it isn't JSON any more: a plain http_500, no 2 MiB code.
	if !errors.As(err, &ae) || ae.Code != "http_500" {
		t.Fatalf("want http_500, got %v", err)
	}
}
