package updsig

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const asset = "Argus-v9.9.9-linux-amd64.tar.gz"

type fixture struct {
	pub   ed25519.PublicKey
	priv  ed25519.PrivateKey
	path  string
	sums  []byte
	sig   []byte
	files map[string][]byte // served by the test server
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	body := []byte("pretend this is the new Argus")
	path := filepath.Join(t.TempDir(), asset)
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256(body)
	sums := []byte(hex.EncodeToString(h[:]) + "  " + asset + "\n" +
		strings.Repeat("ab", 32) + "  argus-desktop_9.9.9_amd64.deb\n")
	sig := Sign(priv, sums)
	return &fixture{pub: pub, priv: priv, path: path, sums: sums, sig: sig,
		files: map[string][]byte{"/" + SumsName: sums, "/" + SigName: sig}}
}

func (f *fixture) server(t *testing.T) *httptest.Server {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") != "application/octet-stream" {
			http.Error(w, "missing Accept header", 400)
			return
		}
		b, ok := f.files[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Write(b)
	}))
	t.Cleanup(s.Close)
	return s
}

var hdr = http.Header{"Accept": {"application/octet-stream"}}

func (f *fixture) verifyHTTP(t *testing.T, pub ed25519.PublicKey) error {
	t.Helper()
	s := f.server(t)
	return VerifyDownload(s.Client(), Keys{pub}, s.URL+"/"+SumsName, s.URL+"/"+SigName, hdr, asset, f.path)
}

func wantFail(t *testing.T, err error, contains string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected a verification failure (%q), got nil", contains)
	}
	if !IsVerifyError(err) {
		t.Fatalf("expected *Error, got %T: %v", err, err)
	}
	if contains != "" && !strings.Contains(err.Error(), contains) {
		t.Fatalf("error %q doesn't mention %q", err, contains)
	}
}

func TestValidSignaturePasses(t *testing.T) {
	f := newFixture(t)
	if err := VerifyFile(Keys{f.pub}, f.sums, f.sig, asset, f.path); err != nil {
		t.Fatal(err)
	}
	if err := f.verifyHTTP(t, f.pub); err != nil {
		t.Fatal(err)
	}
}

func TestTamperedSumsFails(t *testing.T) {
	f := newFixture(t)
	f.files["/"+SumsName] = append([]byte(strings.Repeat("0", 64)+"  evil.exe\n"), f.sums...)
	wantFail(t, f.verifyHTTP(t, f.pub), "signature")
}

func TestWrongKeyFails(t *testing.T) {
	f := newFixture(t)
	other, _, _ := ed25519.GenerateKey(rand.Reader)
	wantFail(t, f.verifyHTTP(t, other), "signature")
}

func TestAssetHashMismatchFails(t *testing.T) {
	f := newFixture(t)
	if err := os.WriteFile(f.path, []byte("swapped binary"), 0o644); err != nil {
		t.Fatal(err)
	}
	wantFail(t, f.verifyHTTP(t, f.pub), "SHA-256")
}

func TestAssetNotListedFails(t *testing.T) {
	f := newFixture(t)
	s := f.server(t)
	err := VerifyDownload(s.Client(), Keys{f.pub}, s.URL+"/"+SumsName, s.URL+"/"+SigName, hdr, "Argus-v9.9.9-macos-universal.dmg", f.path)
	wantFail(t, err, "isn't listed")
}

func TestMissingSigFails(t *testing.T) {
	f := newFixture(t)
	delete(f.files, "/"+SigName)
	wantFail(t, f.verifyHTTP(t, f.pub), "404")
	// release lists no .sig asset at all
	s := f.server(t)
	wantFail(t, VerifyDownload(s.Client(), Keys{f.pub}, s.URL+"/"+SumsName, "", hdr, asset, f.path), "no "+SigName)
	// empty .sig
	wantFail(t, VerifyFile(Keys{f.pub}, f.sums, nil, asset, f.path), "missing")
	// missing SHA256SUMS
	wantFail(t, VerifyDownload(s.Client(), Keys{f.pub}, "", s.URL+"/"+SigName, hdr, asset, f.path), "no "+SumsName)
}

func TestGarbageSigFails(t *testing.T) {
	f := newFixture(t)
	wantFail(t, VerifyFile(Keys{f.pub}, f.sums, []byte("not a signature\n"), asset, f.path), "not a valid")
}

func TestRawSigAccepted(t *testing.T) {
	f := newFixture(t)
	raw := ed25519.Sign(f.priv, f.sums)
	if err := VerifyFile(Keys{f.pub}, f.sums, raw, asset, f.path); err != nil {
		t.Fatal(err)
	}
}

func TestParseSumsFormats(t *testing.T) {
	a, b := strings.Repeat("a", 64), strings.Repeat("B", 64)
	in := "# made by sha256sum\r\n" + a + "  one.tar.gz\r\n\r\n" + b + " *two.exe\r\n" + a + "  name with spaces.dmg"
	got, err := ParseSums([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"one.tar.gz": a, "two.exe": strings.ToLower(b), "name with spaces.dmg": a}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("%s: got %q want %q", k, got[k], v)
		}
	}
	for _, bad := range []string{"", "xyz  file\n", strings.Repeat("g", 64) + "  file\n", a + "  \n", a + "  f\n" + b + "  f\n"} {
		if _, err := ParseSums([]byte(bad)); err == nil {
			t.Errorf("ParseSums(%q) should fail", bad)
		}
	}
}

func TestCRLFSumsVerify(t *testing.T) {
	f := newFixture(t)
	crlf := []byte(strings.ReplaceAll(string(f.sums), "\n", "\r\n"))
	if err := VerifyFile(Keys{f.pub}, crlf, Sign(f.priv, crlf), asset, f.path); err != nil {
		t.Fatal(err)
	}
}

func TestPrivateKeyForms(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	for _, b := range [][]byte{priv.Seed(), priv} {
		k, err := ParsePrivateKey(base64.StdEncoding.EncodeToString(b) + "\n")
		if err != nil || !k.Equal(priv) {
			t.Fatalf("len %d: %v", len(b), err)
		}
	}
	bad := append([]byte{}, priv...)
	bad[40] ^= 1
	if _, err := ParsePrivateKey(base64.StdEncoding.EncodeToString(bad)); err == nil {
		t.Fatal("mismatched public half accepted")
	}
	if _, err := ParsePrivateKey("bm9wZQ=="); err == nil {
		t.Fatal("short key accepted")
	}
}

func TestEmbeddedKeyParses(t *testing.T) {
	ks := DefaultKeys()
	if len(ks) == 0 || len(ks[0]) != ed25519.PublicKeySize {
		t.Fatal("bad embedded key")
	}
}

func TestRotationAcceptsEitherKey(t *testing.T) {
	f := newFixture(t)
	other, _, _ := ed25519.GenerateKey(rand.Reader)
	if err := VerifyFile(Keys{other, f.pub}, f.sums, f.sig, asset, f.path); err != nil {
		t.Fatal(err)
	}
	if !(Keys{other, f.pub}).Has(f.pub) || (Keys{other}).Has(f.pub) {
		t.Fatal("Has is wrong")
	}
	wantFail(t, VerifyFile(Keys{}, f.sums, f.sig, asset, f.path), "signature")
}

// v0.19.6 rotation: both the new key and the previous one are trusted, new one first.
func TestRotationTrustsBoth(t *testing.T) {
	ks := DefaultKeys()
	pub, _ := ParsePublicKey(PublicKey)
	prev, _ := ParsePublicKey(PreviousKey)
	if len(ks) != 2 || !ks.Has(pub) || !ks.Has(prev) || PublicKey == PreviousKey {
		t.Fatalf("want the new and the previous key trusted, got %d keys", len(ks))
	}
}
