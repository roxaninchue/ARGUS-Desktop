package main

import (
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func sleepMs(n int)                       { time.Sleep(time.Duration(n) * time.Millisecond) }
func jsonUnmarshal(b []byte, v any) error { return json.Unmarshal(b, v) }
func writePNG(t *testing.T, p string, w, h int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 200, 255})
		}
	}
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

func TestAttachKinds(t *testing.T) {
	t.Setenv("ARGUS_HOME", t.TempDir())
	d := t.TempDir()
	txt := filepath.Join(d, "notes.md")
	bin := filepath.Join(d, "a.bin")
	img := filepath.Join(d, "shot.png")
	big := filepath.Join(d, "big.png")
	sub := filepath.Join(d, "src")
	_ = os.WriteFile(txt, []byte("# Notes\nhello"), 0o600)
	_ = os.WriteFile(bin, []byte{0, 1, 2, 3, 0, 9}, 0o600)
	_ = os.MkdirAll(filepath.Join(sub, "node_modules", "x"), 0o700)
	_ = os.WriteFile(filepath.Join(sub, "main.go"), []byte("package main"), 0o600)
	_ = os.WriteFile(filepath.Join(sub, "node_modules", "x", "i.js"), []byte("x"), 0o600)
	writePNG(t, img, 300, 200)
	writePNG(t, big, 2400, 1200)

	a := NewApp()
	att := a.AttachPaths([]string{txt, bin, img, big, sub, filepath.Join(d, "missing")})
	if len(att) != 5 {
		t.Fatalf("got %d attachments", len(att))
	}
	kinds := []string{att[0].Kind, att[1].Kind, att[2].Kind, att[3].Kind, att[4].Kind}
	if strings.Join(kinds, ",") != "file,file,image,image,folder" {
		t.Fatalf("kinds: %v", kinds)
	}
	if !strings.HasPrefix(att[2].Thumb, "data:image/jpeg;base64,") || att[2].Send != "" {
		t.Errorf("small image: thumb=%v send=%q", att[2].Thumb != "", att[2].Send)
	}
	if att[3].Send == "" {
		t.Fatal("a 2400px image should get a smaller copy to send")
	}
	f, _ := os.Open(att[3].Send)
	cfg, _, err := image.DecodeConfig(f)
	f.Close()
	if err != nil || cfg.Width != sendSide || cfg.Height != sendSide/2 {
		t.Errorf("sent copy: %v %dx%d", err, cfg.Width, cfg.Height)
	}
	if att[4].Count != 1 {
		t.Errorf("folder count skips node_modules: %d", att[4].Count)
	}

	var b strings.Builder
	b.WriteString("look at these")
	kept := inlineAttach(&b, att)
	msg := b.String()
	if len(kept) != 5 {
		t.Fatalf("kept %d", len(kept))
	}
	if !strings.Contains(msg, `<attached path="`+txt+`" kind="file">`+"\n# Notes\nhello") {
		t.Errorf("text file not inlined:\n%s", msg)
	}
	if !strings.Contains(msg, "binary file") || strings.Contains(msg, "\x00") {
		t.Errorf("binary file should travel as a path only:\n%s", msg)
	}
	if !strings.Contains(msg, `kind="folder">`+"\nmain.go (12 B)") || strings.Contains(msg, "node_modules") {
		t.Errorf("folder tree:\n%s", msg)
	}
	if strings.Contains(msg, "shot.png") {
		t.Errorf("images travel as image parts, not text:\n%s", msg)
	}
	if got := stripAttached(msg); got != "look at these" {
		t.Errorf("stripAttached = %q", got)
	}
}

func TestAttachPasted(t *testing.T) {
	t.Setenv("ARGUS_HOME", t.TempDir())
	d := t.TempDir()
	p := filepath.Join(d, "x.png")
	writePNG(t, p, 40, 30)
	raw, _ := os.ReadFile(p)
	a := NewApp()
	at := a.AttachPasted("image.png", "data:image/png;base64,"+base64.StdEncoding.EncodeToString(raw))
	if at.Kind != "image" || at.Path == "" || at.Thumb == "" {
		t.Fatalf("pasted: %+v", at)
	}
	if bad := a.AttachPasted("x.txt", "data:text/plain;base64,aGk="); bad.Path != "" || bad.Note == "" {
		t.Fatalf("non-image paste: %+v", bad)
	}
}

func TestCleanTranscript(t *testing.T) {
	in := "\n [BLANK_AUDIO]\n And so my fellow Americans,\n ask not (music) what your country can do.\n"
	if got := cleanTranscript(in); got != "And so my fellow Americans, ask not what your country can do." {
		t.Fatalf("got %q", got)
	}
	if got := cleanTranscript(" (eerie music)\n"); got != "" {
		t.Fatalf("noise only: %q", got)
	}
}
