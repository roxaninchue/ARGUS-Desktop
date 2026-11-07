package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lacksonkapapahue/argus-desktop/internal/config"
	"github.com/lacksonkapapahue/argus-desktop/internal/keystore"
)

// Keys saved before ArgusRouter existed (Settings › Providers & keys, or Argus CLI)
// must show up on the dashboard and route without being re-entered.
func TestOldCustomKeysCarryOver(t *testing.T) {
	t.Setenv("ARGUS_HOME", t.TempDir())
	keystore.Keychain = &memKC{m: map[string]string{}}
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		gotAuth = req.Header.Get("Authorization")
		okJSON(w, "from my own service")
	}))
	defer srv.Close()

	// The old way: a custom OpenAI-compatible service plus two Groq keys.
	cfg := config.Load()
	cfg.CustomBase = srv.URL + "/v1"
	cfg.CustomName = "My box"
	_ = cfg.Save()
	if _, err := cfg.SetKey(config.Custom, "sk-mine"); err != nil {
		t.Fatal(err)
	}
	if _, err := cfg.SetKey("groq", "gsk_one"); err != nil {
		t.Fatal(err)
	}
	if _, err := cfg.AddKey("groq", "gsk_two"); err != nil {
		t.Fatal(err)
	}

	r := New(Options{Config: func() *config.Config { return cfg }, Version: "test"})
	var custom, groq *ProviderView
	ps := r.Providers()
	for i := range ps {
		switch ps[i].ID {
		case config.Custom:
			custom = &ps[i]
		case "groq":
			groq = &ps[i]
		}
	}
	if custom == nil || !custom.Ready || custom.Name != "My box" || len(custom.Keys) != 1 || custom.NeedsBase {
		t.Fatalf("custom provider should carry over ready: %+v", custom)
	}
	if strings.Contains(custom.Keys[0].Masked, "sk-mine") {
		t.Fatal("keys are masked on the dashboard")
	}
	if groq == nil || len(groq.Keys) != 2 || !groq.Keys[0].Main {
		t.Fatalf("both groq keys should carry over: %+v", groq)
	}

	rec := post(t, r, "/v1/chat/completions", r.APIKey(), chatBody("custom/my-model", false))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "from my own service") {
		t.Fatalf("custom provider should route: %d %s", rec.Code, rec.Body.String())
	}
	if gotAuth != "Bearer sk-mine" {
		t.Fatalf("old key should be sent upstream, got %q", gotAuth)
	}
}
