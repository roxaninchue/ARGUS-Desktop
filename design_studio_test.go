package main

import (
	"strings"
	"testing"

	"github.com/lacksonkapapahue/argus-desktop/internal/studio"
)

func TestNormStudioMode(t *testing.T) {
	for in, want := range map[string]string{"page": "page", " 3D ": "3d", "Motion": "motion", "audio": "audio", "": "page", "video": "page"} {
		if got := normStudioMode(in); got != want {
			t.Errorf("normStudioMode(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPaletteFor(t *testing.T) {
	b := studio.Brand{Ink: "#111112", Paper: "#F6F5F1", Accents: []string{"#1F4BD8", "#1f4bd8", "nope"}}
	src := `<style>body{color:#111112;background:#fff} .a{color:rgb(255,201,77)} .b{color:#FFC94D}</style>`
	v := paletteFor(b, src)
	if strings.Join(v.Brand, ",") != "#111112,#f6f5f1,#1f4bd8" {
		t.Fatalf("brand swatches: %v", v.Brand)
	}
	// The page's colours, minus the ones the brand already offers, most used first.
	if len(v.Page) != 2 || v.Page[0] != "#ffc94d" || v.Page[1] != "#ffffff" {
		t.Fatalf("page swatches: %v", v.Page)
	}
	if e := paletteFor(studio.Brand{}, ""); e.Brand == nil || e.Page == nil {
		t.Fatal("empty lists, never null, so the wheel can range over them")
	}
}

func TestCleanEdit(t *testing.T) {
	e, err := cleanEdit(studio.Edit{Kind: "style", Props: map[string]string{"translate": "3.6px -1.2px", "color": "#ABCDEF"}})
	if err != nil {
		t.Fatal(err)
	}
	if e.Props["translate"] != "4px -1px" || e.Props["color"] != "#abcdef" {
		t.Fatalf("cleaned: %v", e.Props)
	}
	if _, err := cleanEdit(studio.Edit{Kind: "style", Props: map[string]string{"height": "tall"}}); err == nil {
		t.Fatal("a height that isn't a length should stop the edit")
	}
	txt := studio.Edit{Kind: "text", NewText: "x"}
	if got, _ := cleanEdit(txt); got.NewText != "x" {
		t.Fatal("text edits pass through")
	}
}

func TestTransformSummary(t *testing.T) {
	cases := []struct {
		props map[string]string
		want  string
	}{
		{map[string]string{"translate": "10px 4px"}, "Moved h1.hero to 10px 4px"},
		{map[string]string{"width": "320px", "height": "80px"}, "Resized h1.hero to 320 × 80"},
		{map[string]string{"width": "320px"}, "Resized h1.hero to 320px wide"},
		{map[string]string{"translate": "1px 1px", "width": "2px"}, "Moved and resized h1.hero"},
		{map[string]string{"translate": ""}, "Reset position of h1.hero"},
		{map[string]string{"scale": "1.5"}, "Scaled h1.hero to 1.5×"},
	}
	for _, c := range cases {
		if got := transformSummary("h1.hero", c.props); got != c.want {
			t.Errorf("%v: %q, want %q", c.props, got, c.want)
		}
	}
}
