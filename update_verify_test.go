package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The signing key isn't available to tests, so these cover the app's refusal path;
// the signature and hash checks themselves are tested in internal/updsig.
func TestVerifyUpdateRefusesAndDeletes(t *testing.T) {
	t.Setenv("ARGUS_UPDATE_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("ARGUS_ALLOW_UNSIGNED_UPDATE", "")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/SHA256SUMS":
			w.Write([]byte(strings.Repeat("a", 64) + "  Argus-v9.9.9-linux-amd64.tar.gz\n"))
		case "/SHA256SUMS.sig":
			w.Write([]byte(strings.Repeat("A", 86) + "==\n")) // 64 zero bytes: a well-formed but wrong signature
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	as := ghAsset{Name: "Argus-v9.9.9-linux-amd64.tar.gz"}
	cases := map[string][]ghAsset{
		"no sums or sig": {as},
		"bad sig": {as, {Name: "SHA256SUMS", Download: srv.URL + "/SHA256SUMS"},
			{Name: "SHA256SUMS.sig", Download: srv.URL + "/SHA256SUMS.sig"}},
	}
	for name, assets := range cases {
		file := filepath.Join(t.TempDir(), as.Name)
		os.WriteFile(file, []byte("not really argus"), 0o644)
		err := (&App{}).verifyUpdate(&ghRelease{Tag: "v9.9.9", Assets: assets}, as, file)
		if err == nil || !strings.HasPrefix(err.Error(), unsignedMsg+detailsSep) {
			t.Fatalf("%s: want refusal, got %v", name, err)
		}
		if _, statErr := os.Stat(file); !os.IsNotExist(statErr) {
			t.Fatalf("%s: download wasn't deleted", name)
		}
	}
}

func TestVerifyUpdateDevEscapeHatch(t *testing.T) {
	t.Setenv("ARGUS_UPDATE_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("ARGUS_ALLOW_UNSIGNED_UPDATE", "1")
	as := ghAsset{Name: "Argus-v9.9.9-linux-amd64.tar.gz"}
	file := filepath.Join(t.TempDir(), as.Name)
	os.WriteFile(file, []byte("x"), 0o644)
	if err := (&App{}).verifyUpdate(&ghRelease{Tag: "v9.9.9", Assets: []ghAsset{as}}, as, file); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(file); err != nil {
		t.Fatal("escape hatch deleted the file")
	}
}
