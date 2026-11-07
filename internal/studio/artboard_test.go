package studio

import (
	"context"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestArtboardTemplateRoundTrip(t *testing.T) {
	for _, f := range Formats {
		page := Template(f, "Hello <World>", Brand{Accents: []string{"#ff5500"}, Fonts: []string{"Inter"}})
		got, ok := ParseArtboard(page)
		if !ok || got.ID != f.ID || got.W != f.W || got.H != f.H || got.Print != f.Print {
			t.Fatalf("%s: parsed %+v ok=%v", f.ID, got, ok)
		}
		s := string(page)
		if !strings.Contains(s, "Hello &lt;World&gt;") || !strings.Contains(s, "#ff5500") || !strings.Contains(s, "'Inter'") {
			t.Fatalf("%s: template %s", f.ID, s)
		}
		if (f.Print != "") != strings.Contains(s, "@page") {
			t.Fatalf("%s: @page only for print formats", f.ID)
		}
	}
	if _, ok := ParseArtboard([]byte("<html><head><title>x</title></head></html>")); ok {
		t.Fatal("a web page isn't an artboard")
	}
	if f, ok := ParseArtboard([]byte(`<meta content="640x480" name='argus-artboard'>`)); !ok || f.ID != "custom" || f.W != 640 {
		t.Fatalf("custom size: %+v %v", f, ok)
	}
	if f, ok := ParseArtboard([]byte(`<meta name="argus-artboard" content="slide">`)); !ok || f.W != 1920 || f.H != 1080 {
		t.Fatalf("id only: %+v %v", f, ok)
	}
}

func TestDesignSlug(t *testing.T) {
	for in, want := range map[string]string{"Summer Sale Poster!": "summer-sale-poster", "  ": "design", "Café 2026": "caf-2026"} {
		if got := DesignSlug(in); got != want {
			t.Errorf("DesignSlug(%q) = %q, want %q", in, got, want)
		}
	}
}

// Renders a real artboard with the machine's Chrome/Edge when there is one (CI runners have it).
func TestExportPNGAndPDF(t *testing.T) {
	if FindBrowser() == "" {
		t.Skip("no Chromium-based browser")
	}
	f, _ := FormatByID("post")
	page := Template(f, "Export test", Brand{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write(page)
	}))
	defer srv.Close()
	dir := t.TempDir()
	png1 := filepath.Join(dir, "a.png")
	if err := Export(context.Background(), srv.URL, f, "png", png1); err != nil {
		t.Fatal(err)
	}
	fh, err := os.Open(png1)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := png.DecodeConfig(fh)
	fh.Close()
	if err != nil || cfg.Width != 2*f.W || cfg.Height != 2*f.H {
		t.Fatalf("png %dx%d err=%v, want %dx%d", cfg.Width, cfg.Height, err, 2*f.W, 2*f.H)
	}
	pdf := filepath.Join(dir, "a.pdf")
	if err := Export(context.Background(), srv.URL, f, "pdf", pdf); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(pdf); !strings.HasPrefix(string(b), "%PDF") {
		t.Fatal("not a PDF")
	}
}
