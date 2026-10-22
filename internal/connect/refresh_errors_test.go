package connect

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// v0.19.13: token refresh error paths. A refusal (invalid_grant) means Reconnect; a
// passing failure (offline, timeout, 5xx, rate limit, garbled answer) keeps the tokens
// and the connection, and says so in a sentence.

// tokenSrv is a provider token endpoint whose answer the test picks per call.
type tokenSrv struct {
	srv   *httptest.Server
	calls atomic.Int32
	mu    sync.Mutex
	reply func(w http.ResponseWriter, r *http.Request)
}

func newTokenSrv(t *testing.T) *tokenSrv {
	ts := &tokenSrv{}
	ts.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ts.calls.Add(1)
		ts.mu.Lock()
		f := ts.reply
		ts.mu.Unlock()
		f(w, r)
	}))
	t.Cleanup(ts.srv.Close)
	return ts
}

func (ts *tokenSrv) set(f func(w http.ResponseWriter, r *http.Request)) {
	ts.mu.Lock()
	ts.reply = f
	ts.mu.Unlock()
}

// refreshManager has one OAuth connector "rf" whose token is already expired.
func refreshManager(t *testing.T, tokenURL string) *Manager {
	t.Helper()
	conn := &Connector{ID: "rf", Name: "Refreshy", Auth: AuthOAuth, AuthURL: "http://127.0.0.1:1/authorize", TokenURL: tokenURL, Groups: []string{"rf"}, AnyPort: true}
	m := NewWith(t.TempDir(), &MemSecrets{}, (&recorder{}).host(), []*Connector{conn}, nil)
	m.hc = &http.Client{Timeout: 2 * time.Second}
	t.Cleanup(m.Close)
	_ = m.store.SetToken("rf", &Token{AccessToken: "old-at", RefreshToken: "rt-1", ExpiresAt: time.Now().Add(-time.Minute).UnixMilli()})
	_ = m.store.Update("rf", func(s *ConnState) { s.Connected, s.ClientID = true, "cid" })
	return m
}

func assertKept(t *testing.T, m *Manager, what string) {
	t.Helper()
	if m.store.Conn("rf").Expired {
		t.Fatalf("%s: a passing failure must not mark the connection expired", what)
	}
	if tok := m.store.Token("rf"); tok == nil || tok.RefreshToken != "rt-1" || tok.AccessToken != "old-at" {
		t.Fatalf("%s: tokens must be kept, got %+v", what, tok)
	}
}

func TestRefreshSuccessKeepsRefreshTokenAndScopes(t *testing.T) {
	ts := newTokenSrv(t)
	ts.set(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.Form.Get("grant_type") != "refresh_token" || r.Form.Get("refresh_token") != "rt-1" || r.Form.Get("client_id") != "cid" {
			w.WriteHeader(400)
			return
		}
		io.WriteString(w, `{"access_token":"new-at","expires_in":"3600","scope":"a b"}`) // expires_in as a string
	})
	m := refreshManager(t, ts.srv.URL)
	tok, err := m.access(context.Background(), "rf", false)
	if err != nil || tok != "new-at" {
		t.Fatalf("refresh: %q %v", tok, err)
	}
	got := m.store.Token("rf")
	if got.RefreshToken != "rt-1" || got.ExpiresAt < time.Now().Add(59*time.Minute).UnixMilli() {
		t.Fatalf("stored: %+v", got)
	}
	if sc := m.store.Conn("rf").Scopes; len(sc) != 2 {
		t.Fatalf("scopes: %v", sc)
	}
	// Not due any more: no second call.
	if _, err := m.access(context.Background(), "rf", false); err != nil || ts.calls.Load() != 1 {
		t.Fatalf("second access refreshed again (calls=%d, err=%v)", ts.calls.Load(), err)
	}
}

func TestRefreshRefusedMeansReauth(t *testing.T) {
	ts := newTokenSrv(t)
	ts.set(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		io.WriteString(w, `{"error":"invalid_grant","error_description":"revoked"}`)
	})
	m := refreshManager(t, ts.srv.URL)
	if _, err := m.access(context.Background(), "rf", false); !errors.Is(err, ErrReauth) {
		t.Fatalf("want ErrReauth, got %v", err)
	}
	if !m.store.Conn("rf").Expired {
		t.Fatal("should be expired")
	}
	// Expired: no more calls to the provider, no panic.
	before := ts.calls.Load()
	if _, err := m.access(context.Background(), "rf", true); !errors.Is(err, ErrReauth) || ts.calls.Load() != before {
		t.Fatalf("expired connector must not refresh again: %v", err)
	}
}

func TestRefreshPassingFailuresKeepConnection(t *testing.T) {
	cases := map[string]func(w http.ResponseWriter, r *http.Request){
		"503 html": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(503)
			io.WriteString(w, "<html><h1>Service Unavailable</h1></html>")
		},
		"500 empty": func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(500) },
		"429": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(429)
			io.WriteString(w, `{"message":"slow down"}`)
		},
		"200 html":   func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "<html>captive portal</html>") },
		"200 broken": func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, `{"access_token":"x`) },
		"server_error code": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(400)
			io.WriteString(w, `{"error":"temporarily_unavailable"}`)
		},
	}
	for name, h := range cases {
		ts := newTokenSrv(t)
		ts.set(h)
		m := refreshManager(t, ts.srv.URL)
		_, err := m.access(context.Background(), "rf", false)
		if err == nil || errors.Is(err, ErrReauth) {
			t.Fatalf("%s: want a passing error, got %v", name, err)
		}
		if !strings.Contains(err.Error(), "Try again") || strings.Contains(err.Error(), "<html") {
			t.Fatalf("%s: message %q", name, err)
		}
		assertKept(t, m, name)
	}
}

// dnsFail is a transport whose every lookup fails like an offline resolver (no proxy:
// the sandbox's HTTP proxy would otherwise answer for any host).
func dnsFail() *http.Transport {
	return &http.Transport{DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
		return nil, &net.DNSError{Err: "no such host", Name: addr, IsNotFound: true}
	}}
}

func TestRefreshOfflineKeepsTokens(t *testing.T) {
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	refused := "http://" + ln.Addr().String() + "/token"
	ln.Close()

	block := make(chan struct{})
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-block:
		case <-r.Context().Done():
		}
	}))
	t.Cleanup(slow.Close)
	defer close(block)

	for name, u := range map[string]string{
		"refused": refused,
		"dns":     "http://token.does-not-exist.invalid/token",
		"timeout": slow.URL,
	} {
		m := refreshManager(t, u)
		m.hc = &http.Client{Timeout: 300 * time.Millisecond}
		if name == "dns" {
			m.hc.Transport = dnsFail()
		}
		_, err := m.access(context.Background(), "rf", false)
		if err == nil || errors.Is(err, ErrReauth) || !strings.Contains(err.Error(), "couldn't reach Refreshy") {
			t.Fatalf("%s: got %v", name, err)
		}
		assertKept(t, m, name)
	}
}

func TestRefreshClockEdges(t *testing.T) {
	ts := newTokenSrv(t)
	ts.set(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"access_token":"new-at","expires_in":3600}`)
	})
	m := refreshManager(t, ts.srv.URL)
	set := func(exp int64) {
		_ = m.store.SetToken("rf", &Token{AccessToken: "old-at", RefreshToken: "rt-1", ExpiresAt: exp})
	}
	// Expires in 30 s (inside the 60 s skew margin): refreshed early.
	set(time.Now().Add(30 * time.Second).UnixMilli())
	if tok, _ := m.access(context.Background(), "rf", false); tok != "new-at" || ts.calls.Load() != 1 {
		t.Fatalf("near expiry should refresh: %q calls=%d", tok, ts.calls.Load())
	}
	// Expires in 5 minutes: used as is.
	set(time.Now().Add(5 * time.Minute).UnixMilli())
	if tok, _ := m.access(context.Background(), "rf", false); tok != "old-at" || ts.calls.Load() != 1 {
		t.Fatalf("fresh token should be used: %q calls=%d", tok, ts.calls.Load())
	}
	// No expiry known: used until the API says 401.
	set(0)
	if tok, _ := m.access(context.Background(), "rf", false); tok != "old-at" || ts.calls.Load() != 1 {
		t.Fatalf("unknown expiry should be used: %q", tok)
	}
	// An expiry far in the past (a clock that jumped): refreshed, no crash.
	set(1)
	if tok, _ := m.access(context.Background(), "rf", false); tok != "new-at" {
		t.Fatalf("ancient expiry: %q", tok)
	}
	// No expires_in in the answer: stored with no expiry, not "expired at epoch".
	ts.set(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, `{"access_token":"noexp"}`) })
	set(1)
	if tok, _ := m.access(context.Background(), "rf", false); tok != "noexp" || m.store.Token("rf").ExpiresAt != 0 {
		t.Fatalf("no expires_in: %q %+v", tok, m.store.Token("rf"))
	}
}

func TestRefreshConcurrentDedupe(t *testing.T) {
	ts := newTokenSrv(t)
	ts.set(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		io.WriteString(w, `{"access_token":"new-at","expires_in":3600}`)
	})
	m := refreshManager(t, ts.srv.URL)
	var wg sync.WaitGroup
	got := make([]string, 8)
	for i := range got {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			got[i], _ = m.access(context.Background(), "rf", false)
		}(i)
	}
	wg.Wait()
	if n := ts.calls.Load(); n != 1 {
		t.Fatalf("8 callers with an expired token should refresh once, got %d", n)
	}
	for i, g := range got {
		if g != "new-at" {
			t.Fatalf("caller %d got %q", i, g)
		}
	}

	// Several 401s for the same stale token (renew): one refresh, the rest reuse it.
	_ = m.store.SetToken("rf", &Token{AccessToken: "stale", RefreshToken: "rt-1"})
	ts.calls.Store(0)
	ts.set(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		io.WriteString(w, `{"access_token":"fresh","expires_in":3600}`)
	})
	for i := range got {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			got[i], _ = m.renew(context.Background(), "rf", "stale")
		}(i)
	}
	wg.Wait()
	if n := ts.calls.Load(); n != 1 {
		t.Fatalf("8 concurrent 401s should refresh once, got %d", n)
	}
	for i, g := range got {
		if g != "fresh" {
			t.Fatalf("renew %d got %q", i, g)
		}
	}
}

func TestUnauthorizedRefreshesAndRetriesOnce(t *testing.T) {
	var api atomic.Int32
	var always401 atomic.Bool
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"access_token":"good","expires_in":3600}`)
	})
	mux.HandleFunc("/api", func(w http.ResponseWriter, r *http.Request) {
		api.Add(1)
		if always401.Load() || r.Header.Get("Authorization") != "Bearer good" {
			w.WriteHeader(401)
			return
		}
		io.WriteString(w, `{"ok":true}`)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	m := refreshManager(t, srv.URL+"/token")
	_ = m.store.SetToken("rf", &Token{AccessToken: "revoked", RefreshToken: "rt-1", ExpiresAt: time.Now().Add(time.Hour).UnixMilli()})

	c := &call{ctx: context.Background(), m: m, conn: "rf", token: "revoked"}
	var out struct{ OK bool }
	if err := c.get(srv.URL+"/api", &out); err != nil || !out.OK || api.Load() != 2 {
		t.Fatalf("401 → refresh → retry: err=%v ok=%v calls=%d", err, out.OK, api.Load())
	}

	// Still 401 after a fresh token: exactly one retry, then Reconnect.
	always401.Store(true)
	api.Store(0)
	c = &call{ctx: context.Background(), m: m, conn: "rf", token: "good"}
	if err := c.get(srv.URL+"/api", &out); !errors.Is(err, ErrReauth) || api.Load() != 2 {
		t.Fatalf("second 401: err=%v calls=%d", err, api.Load())
	}
	if !m.store.Conn("rf").Expired {
		t.Fatal("a 401 after refreshing should mark the connection expired")
	}
}

// failBroker refuses every refresh with err.
type failBroker struct {
	fakeBroker
	err error
}

func (b *failBroker) Refresh(ctx context.Context, id, rt string) (*Token, error) { return nil, b.err }

func TestBrokerRefreshErrors(t *testing.T) {
	for name, tc := range map[string]struct {
		err     error
		expired bool
	}{
		"unreachable": {ErrBrokerUnreachable, false},
		"temporary":   {ErrBrokerTemporary, false},
		"wrapped tmp": {errors.Join(ErrBrokerTemporary, errors.New("http_502")), false},
		"deadline":    {context.DeadlineExceeded, false},
		"refused":     {errors.New("invalid_grant"), true},
	} {
		rec := &recorder{}
		m := NewWith(t.TempDir(), &MemSecrets{}, rec.host(), Catalog, BuiltinTools())
		m.SetBroker(&failBroker{fakeBroker: fakeBroker{on: map[string]bool{"slack": true}}, err: tc.err})
		_ = m.store.SetToken("slack", &Token{AccessToken: "a", RefreshToken: "r", ExpiresAt: time.Now().Add(-time.Minute).UnixMilli()})
		_ = m.store.Update("slack", func(s *ConnState) { s.Connected, s.Via = true, "broker" })
		_, err := m.access(context.Background(), "slack", false)
		if got := m.store.Conn("slack").Expired; got != tc.expired {
			t.Fatalf("%s: expired=%v, want %v (err %v)", name, got, tc.expired, err)
		}
		if tc.expired != errors.Is(err, ErrReauth) {
			t.Fatalf("%s: err %v", name, err)
		}
		if tok := m.store.Token("slack"); tok == nil || tok.RefreshToken != "r" {
			t.Fatalf("%s: refresh token must never be wiped by a failed refresh: %+v", name, tok)
		}
		m.Close()
	}
}
