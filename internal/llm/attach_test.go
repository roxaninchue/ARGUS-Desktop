package llm

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lacksonkapapahue/argus-desktop/internal/config"
)

func TestWireImages(t *testing.T) {
	p := filepath.Join(t.TempDir(), "s.png")
	_ = os.WriteFile(p, []byte("\x89PNG fake"), 0o600)
	msgs := []Message{{Role: "user", Content: "what's this?", Attach: []Attach{{Kind: "image", Path: p, Name: "s.png"}, {Kind: "file", Path: "/x.txt"}}}}

	w := wire(msgs, true, false)
	parts, ok := w[0].Content.([]map[string]any)
	if !ok || len(parts) != 2 || parts[0]["type"] != "text" || parts[1]["type"] != "image_url" {
		t.Fatalf("vision model content: %#v", w[0].Content)
	}
	if u := parts[1]["image_url"].(map[string]any)["url"].(string); !strings.HasPrefix(u, "data:image/png;base64,") {
		t.Fatalf("image url: %.40s", u)
	}

	w = wire(msgs, false, false)
	s, ok := w[0].Content.(string)
	if !ok || !strings.Contains(s, "can't see images") || !strings.Contains(s, p) {
		t.Fatalf("non-vision fallback: %#v", w[0].Content)
	}

	plain := wire([]Message{{Role: "user", Content: "hi"}}, true, false)
	if plain[0].Content != "hi" {
		t.Fatalf("plain message changed: %#v", plain[0].Content)
	}
}

func TestKeepRecentImages(t *testing.T) {
	var msgs []Message
	for i := 0; i < 4; i++ {
		msgs = append(msgs, Message{Role: "user", Content: "m", Attach: []Attach{{Kind: "image", Path: "/i.png"}}})
	}
	out := KeepRecentImages(msgs, 2)
	if imageCount(out[0]) != 0 || imageCount(out[1]) != 0 || imageCount(out[2]) != 1 || imageCount(out[3]) != 1 {
		t.Fatal("only the last two image messages should keep their images")
	}
	if imageCount(msgs[0]) != 1 {
		t.Fatal("KeepRecentImages changed the saved history")
	}
}

func TestPrivateVisionNeverSendsPixels(t *testing.T) {
	p := filepath.Join(t.TempDir(), "s.png")
	_ = os.WriteFile(p, []byte("\x89PNG fake"), 0o600)
	old := describeImage
	calls := 0
	describeImage = func(_ context.Context, path string) string { calls++; return "Text: Error 404" }
	defer func() { describeImage = old }()
	msgs := []Message{{Role: "user", Content: "what's this?", Attach: []Attach{{Kind: "image", Path: p, Name: "s.png"}}}}
	var said string
	prep := PrepareImages(context.Background(), msgs, func(s string) { said = s })
	if !prep[0].Attach[0].Private || prep[0].Attach[0].Desc != "Text: Error 404" || said == "" {
		t.Fatalf("prepared %+v (status %q)", prep[0].Attach[0], said)
	}
	if msgs[0].Attach[0].Private {
		t.Fatal("PrepareImages changed the caller's messages")
	}
	// A cloud model that can see still gets only text.
	w := wire(prep, true, false)
	s, ok := w[0].Content.(string)
	if !ok || !strings.Contains(s, "Error 404") || strings.Contains(s, "base64") {
		t.Fatalf("cloud content: %#v", w[0].Content)
	}
	// A local vision model gets the picture.
	if parts, ok := wire(prep, true, true)[0].Content.([]map[string]any); !ok || len(parts) != 2 {
		t.Fatalf("local content: %#v", wire(prep, true, true)[0].Content)
	}
	// Read once, then remembered.
	PrepareImages(context.Background(), msgs, nil)
	if calls != 1 {
		t.Fatalf("read %d times", calls)
	}
}

func TestIsLocal(t *testing.T) {
	for base, want := range map[string]bool{"http://127.0.0.1:1234/v1": true, "http://localhost:8080/v1": true, "http://[::1]:9/v1": true, "https://api.openai.com/v1": false, "": false} {
		if got := IsLocal(config.Provider{ID: "custom", BaseURL: base}); got != want {
			t.Errorf("IsLocal(%q) = %v", base, got)
		}
	}
	if !IsLocal(config.Provider{ID: "ollama"}) {
		t.Error("ollama is local")
	}
}
