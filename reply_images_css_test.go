package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// cssEffective returns the last value chat.css gives prop for an exact selector (same-specificity
// rules: the later one wins, which is how these reply picture rules are written).
func cssEffective(css, sel, prop string) string {
	css = regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(css, "")
	val := ""
	for _, m := range regexp.MustCompile(`([^{}]+)\{([^{}]*)\}`).FindAllStringSubmatch(css, -1) {
		hit := false
		for _, s := range strings.Split(m[1], ",") {
			if strings.Join(strings.Fields(s), " ") == sel {
				hit = true
			}
		}
		if !hit {
			continue
		}
		for _, d := range strings.Split(m[2], ";") {
			if k, v, ok := strings.Cut(d, ":"); ok && strings.TrimSpace(k) == prop {
				val = strings.Join(strings.Fields(v), " ")
			}
		}
	}
	return val
}

// v0.19.15: a single picture in a reply (a CSV chart) is shown whole; 2-3 side by side and the deck stay cropped tiles.
func TestReplySingleImageNotCropped(t *testing.T) {
	b, err := os.ReadFile("frontend/dist/chat.css")
	if err != nil {
		t.Fatal(err)
	}
	css := string(b)
	for _, sel := range []string{".md .rimg img", ".md .rside img"} {
		if v := cssEffective(css, sel, "object-fit"); v != "contain" {
			t.Errorf("%s object-fit = %q, want contain", sel, v)
		}
		if v := cssEffective(css, sel, "aspect-ratio"); v != "auto" {
			t.Errorf("%s aspect-ratio = %q, want auto", sel, v)
		}
		if v := cssEffective(css, sel, "height"); v != "auto" {
			t.Errorf("%s height = %q, want auto", sel, v)
		}
	}
	if v := cssEffective(css, ".md .rgroup img", "object-fit"); v != "cover" {
		t.Errorf("group object-fit = %q, want cover", v)
	}
	if v := cssEffective(css, ".md .rgroup img", "aspect-ratio"); v != "4 / 3" {
		t.Errorf("group aspect-ratio = %q, want 4 / 3", v)
	}
	if v := cssEffective(css, ".md .deck .dcard img", "object-fit"); v != "cover" {
		t.Errorf("deck object-fit = %q, want cover", v)
	}
	// md.js still renders one picture on its own as <figure class="rimg">
	js, err := os.ReadFile("frontend/dist/md.js")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(js), `<figure class="rimg">`) {
		t.Error(`md.js no longer emits <figure class="rimg"> for a single picture`)
	}
}
