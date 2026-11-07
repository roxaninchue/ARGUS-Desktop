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
		w.Write([]byte(`{"version":"9.9.9","name":"Argus","system":"You are Argus, test."}`))
	}))
	defer srv.Close()
	old := URL
	URL = srv.URL
	defer func() { URL = old }()
	dir := t.TempDir()
	if err := Refresh(context.Background(), dir, srv.Client()); err != nil {
		t.Fatal(err)
	}
	if System() != "You are Argus, test." {
		t.Fatalf("refresh: %q", System())
	}
	// A bad cache is ignored; a fresh cache isn't refetched.
	os.WriteFile(filepath.Join(dir, "persona.json"), []byte("nope"), 0o644)
	Load(dir)
	if System() != "You are Argus, test." {
		t.Fatal("bad cache replaced the persona")
	}
}

// v0.19.13: persona 1.0.2's evals mix a message list and a plain string under "context".
func TestParsesPersona102Shapes(t *testing.T) {
	b := []byte(`{"version":"1.0.2","name":"Argus","system":"You are Argus.","evals":[{"id":"a","context":[{"role":"assistant","content":"hi"}],"user":"x","expect":["y"]},{"id":"b","context":"device context with weather","user":"x","expect":["y"]}],"eval_runner":"r","context_block":"c"}`)
	p, err := parse(b)
	if err != nil || p.Version != "1.0.2" || len(p.Evals) != 2 {
		t.Fatalf("1.0.2 shapes: %v %+v", err, p)
	}
	if bp, err := parse(bundled); err != nil || bp.Version != "1.0.2" {
		t.Fatalf("bundled persona should be 1.0.2: %v %s", err, bp.Version)
	}
}
