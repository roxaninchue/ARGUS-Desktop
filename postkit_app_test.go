package main

import (
	"context"
	"encoding/base64"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lacksonkapapahue/argus-desktop/internal/config"
	"github.com/lacksonkapapahue/argus-desktop/internal/llm"
	"github.com/lacksonkapapahue/argus-desktop/internal/postkit"
)

// The model is stubbed at the app's chat call, so this covers the real path: request, photo
// attached as a chat image, reply, rules, validation, JSON, export. The stub's route names a
// provider and model on purpose: none of it may reach the result.
func stubPostKitModel(t *testing.T, reply string, err error) *[][]llm.Message {
	t.Helper()
	var seen [][]llm.Message
	old := postkitComplete
	postkitComplete = func(ctx context.Context, cfg *config.Config, msgs []llm.Message, tools []llm.Tool, o llm.Opts) (*llm.Result, error) {
		seen = append(seen, msgs)
		if err != nil {
			return nil, err
		}
		return &llm.Result{Text: reply, Provider: "groq", Model: "llama-3.3-70b", Route: "Groq · llama-3.3-70b", Note: "Gemini was rate limited, Groq answered"}, nil
	}
	t.Cleanup(func() { postkitComplete = old })
	return &seen
}

func noRouteNames(t *testing.T, s string) {
	t.Helper()
	for _, w := range []string{"groq", "llama", "gemini"} {
		if strings.Contains(strings.ToLower(s), w) {
			t.Fatalf("result names %q: %s", w, s)
		}
	}
}

func TestPostKitIdeaEndToEnd(t *testing.T) {
	seen := stubPostKitModel(t, `{"hook":"Three ingredients, ten minutes, zero excuses","hashtags":["#quickdinner","#nshima","#cooking"],
"alt_text":"A plate of nshima with greens and tomato relish on a wooden table",
"posts":{"x":{"body":"My weeknight fix. Recipe: https://argus.test/r"},"tiktok":{"body":"Weeknight dinner in ten. Follow for part 2."}}}`, nil)
	a := &App{cfg: &config.Config{}}
	r := a.PostKitMake(PostKitRequest{Idea: "Quick nshima dinner, recipe at https://argus.test/r", Language: "en", Platforms: []string{"x", "tiktok"}})
	if r.Error != "" || r.Kit == nil {
		t.Fatalf("no kit: %+v", r)
	}
	if len(*seen) != 1 || len((*seen)[0]) != 2 || (*seen)[0][0].Role != "system" || len((*seen)[0][1].Attach) != 0 {
		t.Fatalf("model call: %+v", *seen)
	}
	k, problems := postkit.Parse([]byte(r.JSON))
	if k == nil || len(problems) > 0 {
		t.Fatalf("JSON isn't a valid kit: %v\n%s", problems, r.JSON)
	}
	if !strings.Contains(k.Platforms["x"].Caption, "https://argus.test/r") {
		t.Fatal("link lost")
	}
	noRouteNames(t, r.JSON+r.Text+r.Note+r.Error)
	// Re-checking the same JSON (Open a kit) agrees.
	if c := a.PostKitCheck(r.JSON); c.Kit == nil || len(c.Problems) > 0 {
		t.Fatalf("check: %+v", c.Problems)
	}
	if c := a.PostKitCheck(`{"schema":"argus.postkit/1"}`); len(c.Problems) == 0 {
		t.Fatal("broken kit passed the check")
	}
}

func TestPostKitPhotoEndToEnd(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", os.Getenv("HOME"))
	img := image.NewRGBA(image.Rect(0, 0, 900, 1600))
	for y := 0; y < 1600; y += 4 {
		for x := 0; x < 900; x += 4 {
			img.Set(x, y, color.RGBA{200, uint8(x % 255), uint8(y % 255), 255})
		}
	}
	photo := filepath.Join(t.TempDir(), "sunset.jpg")
	f, _ := os.Create(photo)
	jpeg.Encode(f, img, nil)
	f.Close()
	seen := stubPostKitModel(t, "```json\n"+`{"hook":"Golden hour on the Zambezi","hashtags":["#Zambezi","#goldenhour","#travel"],
"alt_text":"A wooden boat on a wide river at sunset, the sky orange and purple","focus":{"x":0.5,"y":0.6},
"posts":{"instagram":{"body":"Some evenings you just stop and watch."},"reddit":{"title":"Sunset on the Zambezi from a small boat","body":"Taken last week near Livingstone."}}}`+"\n```", nil)
	a := &App{cfg: &config.Config{}}
	ph := a.PostKitPhotoPath(photo)
	if ph.Error != "" || ph.Width != 900 || ph.Height != 1600 {
		t.Fatalf("photo: %+v", ph)
	}
	r := a.PostKitMake(PostKitRequest{Photo: photo, Language: "en", Platforms: []string{"instagram", "reddit"}})
	if r.Error != "" || r.Kit == nil {
		t.Fatalf("no kit: %+v", r)
	}
	um := (*seen)[0][1]
	if len(um.Attach) != 1 || um.Attach[0].Kind != "image" || um.Attach[0].Path != photo {
		t.Fatalf("photo not attached like a chat image: %+v", um.Attach)
	}
	if k, p := postkit.Parse([]byte(r.JSON)); k == nil || len(p) > 0 || k.Source.Width != 900 || len(k.Crops) != 4 {
		t.Fatalf("kit: %v %+v", p, k)
	}
	noRouteNames(t, r.JSON+r.Text)
	dir := t.TempDir()
	ex := postKitExportTo(dir, r.JSON, photo)
	if ex.Error != "" || len(ex.Files) != 6 {
		t.Fatalf("export: %+v", ex)
	}
	if k, p := postkit.Parse([]byte(ex.JSON)); k == nil || len(p) > 0 || k.Crops[2].File != "sunset-9x16.jpg" {
		t.Fatalf("exported kit: %v", p)
	}
	if _, err := os.Stat(filepath.Join(dir, "sunset-9x16.jpg")); err != nil {
		t.Fatal(err)
	}
}

func TestPostKitErrorsArePlain(t *testing.T) {
	stubPostKitModel(t, "", errors.New("groq: 503 from llama-3.3-70b"))
	a := &App{cfg: &config.Config{}}
	r := a.PostKitMake(PostKitRequest{Idea: "hello", Platforms: []string{"x"}})
	if r.Kit != nil || !strings.Contains(r.Error, "couldn't reach") {
		t.Fatalf("%+v", r)
	}
	noRouteNames(t, r.Error)
	if r := a.PostKitMake(PostKitRequest{Platforms: []string{"x"}}); !strings.Contains(r.Error, "Add a photo or type an idea") {
		t.Fatalf("empty: %q", r.Error)
	}
	if r := a.PostKitMake(PostKitRequest{Photo: "/nope.jpg", Platforms: []string{"x"}}); r.Error == "" {
		t.Fatal("missing photo accepted")
	}
}

// tinyWebP is a 120x80 lossy WebP (left half red, right half blue).
const tinyWebP = "UklGRpIAAABXRUJQVlA4IIYAAADQBwCdASp4AFAAPm02mEikIyKhJNK4AIANiWcAlAD8QNNL+KtGA/ACu/9wAAC0eEMkeYzsmJmb7gHTSbxnYvGw87JiZm+wgAD+694vQX20ICNgr8Ej+Wh+Wh+3ufVtDF54r+VSy/qZK2G+3NnAP9JbycuV1XG4A6iZeLiMK9rnY1zsa4AAAA=="

// v0.19.13 regression: a WebP photo (what most images saved from the web are) was refused with
// "Use a JPG, PNG or GIF photo." although the drop zone and paste accept any image.
func TestPostKitWebPPhoto(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", os.Getenv("HOME"))
	b, _ := base64.StdEncoding.DecodeString(tinyWebP)
	photo := filepath.Join(t.TempDir(), "saved-from-web.webp")
	if err := os.WriteFile(photo, b, 0o644); err != nil {
		t.Fatal(err)
	}
	seen := stubPostKitModel(t, `{"hook":"Two colours, one frame","hashtags":["#colour","#minimal","#design"],
"alt_text":"A rectangle split down the middle, red on the left and blue on the right","focus":{"x":0.5,"y":0.5},
"posts":{"instagram":{"body":"Sometimes two colours say enough."}}}`, nil)
	a := &App{cfg: &config.Config{}}
	ph := a.PostKitPhotoPath(photo)
	if ph.Error != "" || ph.Width != 120 || ph.Height != 80 {
		t.Fatalf("webp photo refused: %+v", ph)
	}
	r := a.PostKitMake(PostKitRequest{Photo: photo, Language: "en", Platforms: []string{"instagram"}})
	if r.Error != "" || r.Kit == nil {
		t.Fatalf("no kit from a webp photo: %+v", r)
	}
	if um := (*seen)[0][1]; len(um.Attach) != 1 || um.Attach[0].Kind != "image" {
		t.Fatalf("webp photo not attached: %+v", um.Attach)
	}
	ex := postKitExportTo(t.TempDir(), r.JSON, photo)
	if ex.Error != "" || len(ex.Files) != 6 {
		t.Fatalf("export from webp: %+v", ex)
	}
	// pasted as a data URL, the way the Post kit view sends a pasted or dropped file
	pp := a.PostKitPhotoPasted("pasted.webp", "data:image/webp;base64,"+tinyWebP)
	if pp.Error != "" || pp.Width != 120 {
		t.Fatalf("pasted webp refused: %+v", pp)
	}
}
