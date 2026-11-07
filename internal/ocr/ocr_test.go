package ocr

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writePNG(t *testing.T, w, h int, fill func(x, y int) color.Color) string {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, fill(x, y))
		}
	}
	p := filepath.Join(t.TempDir(), "i.png")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestSummary(t *testing.T) {
	p := writePNG(t, 200, 100, func(x, y int) color.Color {
		if x < 150 {
			return color.NRGBA{0x10, 0x10, 0x10, 0xff}
		}
		return color.NRGBA{0x66, 0x88, 0xff, 0xff}
	})
	s := Summary(p)
	if !strings.Contains(s, "200×100 px") || !strings.Contains(s, "mostly dark") || !strings.Contains(s, "#111111 75%") || !strings.Contains(s, "#6688ff 25%") {
		t.Fatalf("summary %q", s)
	}
	if Summary(filepath.Join(t.TempDir(), "missing.png")) != "" {
		t.Fatal("missing file should give no summary")
	}
}

func TestDescribeNeverFails(t *testing.T) {
	p := writePNG(t, 40, 40, func(x, y int) color.Color { return color.White })
	d := Describe(context.Background(), p)
	if !strings.Contains(d, "40×40 px") || !strings.Contains(d, "Text") {
		t.Fatalf("describe %q", d)
	}
	t.Logf("engine-dependent description: %s", d)
}

// On Windows and macOS (and Linux with tesseract) the OS engine must read real text.
func TestOSEngineReadsText(t *testing.T) {
	if name, _ := Engine(); name == "" {
		t.Skip("no OCR engine on this computer")
	}
	txt, err := Text(context.Background(), filepath.Join("testdata", "hello.png"))
	if err == ErrNoEngine {
		t.Skip("engine present but no OCR language installed")
	}
	if err != nil {
		t.Fatal(err)
	}
	low := strings.ToLower(txt)
	if !strings.Contains(low, "argus") || !strings.Contains(low, "404") {
		t.Fatalf("OCR read %q", txt)
	}
}
