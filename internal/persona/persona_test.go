package persona

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestBundledAndRefresh(t *testing.T) {
	if p := Current(); p.Version == "" || p.System == "" || len(p.Evals) < 5 {
		t.Fatalf("bundled persona: %+v", p)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"version":"9.9.9","name":"Blaze","system":"You are Blaze, test."}`))
	}))
	defer srv.Close()
	old := URL
	URL = srv.URL
	defer func() { URL = old }()
	dir := t.TempDir()
	if err := Refresh(context.Background(), dir, srv.Client()); err != nil {
		t.Fatal(err)
	}
	if System() != "You are Blaze, test." {
		t.Fatalf("refresh: %q", System())
	}
	// A bad cache is ignored; a fresh cache isn't refetched.
	os.WriteFile(filepath.Join(dir, "persona.json"), []byte("nope"), 0o644)
	Load(dir)
	if System() != "You are Blaze, test." {
		t.Fatal("bad cache replaced the persona")
	}
}
