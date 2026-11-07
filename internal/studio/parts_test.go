package studio

import (
	"encoding/json"
	"strings"
	"testing"
)

const mockup = `<!doctype html><html><body><canvas></canvas>
<script type="application/json" id="argus-parts">
{
  "units": "mm",
  "parts": [
    { "id": "seat", "label": "Seat", "shape": "box", "size": [400, 40, 400], "position": [0, 420, 0], "color": "#8a5a3c" },
    { "id": "leg-fl", "label": "Front left leg", "shape": "cylinder", "radius": 18, "height": 400, "position": [-170, 200, 170], "color": "#333333" }
  ]
}
</script>
<script type="module">/* builds the scene */</script>
</body></html>`

func part(t *testing.T, src, id string) map[string]any {
	t.Helper()
	m := partsBlock.FindStringSubmatch(src)
	if m == nil {
		t.Fatal("no parts block")
	}
	var top struct{ Parts []map[string]any }
	if err := json.Unmarshal([]byte(m[1]), &top); err != nil {
		t.Fatal(err)
	}
	for _, p := range top.Parts {
		if p["id"] == id {
			return p
		}
	}
	t.Fatalf("no part %s", id)
	return nil
}

func TestApplyPartReplacesOnlyThatPart(t *testing.T) {
	out, err := Apply(mockup, LangHTML, Edit{Kind: "part", Target: Target{Tag: "part", ID: "seat"}, Props: map[string]string{"color": `"#c0392b"`, "position": `[0,450,0]`}})
	if err != nil {
		t.Fatal(err)
	}
	p := part(t, out, "seat")
	if p["color"] != "#c0392b" || p["position"].([]any)[1].(float64) != 450 {
		t.Fatalf("seat not updated: %v", p)
	}
	if part(t, out, "leg-fl")["color"] != "#333333" {
		t.Fatal("another part changed")
	}
	if !strings.Contains(out, `"position": [0, 450, 0]`) {
		t.Fatalf("array not written compactly:\n%s", out)
	}
	// Only one line of the file changes.
	a, b := strings.Split(mockup, "\n"), strings.Split(out, "\n")
	diff := 0
	for i := range a {
		if a[i] != b[i] {
			diff++
		}
	}
	if len(a) != len(b) || diff != 1 {
		t.Fatalf("want a one-line change, got %d lines changed", diff)
	}
}

func TestApplyPartAddsAndRemovesKeys(t *testing.T) {
	out, err := ApplyPart(mockup, "leg-fl", map[string]string{"rotation": "[0, 45, 0]"})
	if err != nil {
		t.Fatal(err)
	}
	if r := part(t, out, "leg-fl")["rotation"].([]any); r[1].(float64) != 45 {
		t.Fatalf("rotation not added: %v", r)
	}
	out, err = ApplyPart(out, "leg-fl", map[string]string{"rotation": "", "color": ""})
	if err != nil {
		t.Fatal(err)
	}
	p := part(t, out, "leg-fl")
	if _, ok := p["rotation"]; ok {
		t.Fatal("rotation not removed")
	}
	if _, ok := p["color"]; ok {
		t.Fatal("color not removed")
	}
}

func TestApplyPartRefuses(t *testing.T) {
	if _, err := ApplyPart(mockup, "nope", map[string]string{"color": `"#fff"`}); err != ErrNotFound {
		t.Fatalf("unknown part: %v", err)
	}
	if _, err := ApplyPart(mockup, "seat", map[string]string{"onclick": `"x"`}); err == nil {
		t.Fatal("unknown key accepted")
	}
	if _, err := ApplyPart(mockup, "seat", map[string]string{"color": `#fff`}); err == nil {
		t.Fatal("invalid JSON accepted")
	}
	if _, err := ApplyPart("<html></html>", "seat", map[string]string{"color": `"#fff"`}); err != ErrNotFound {
		t.Fatal("page without parts")
	}
	if !HasParts(mockup) || HasParts("<html></html>") {
		t.Fatal("HasParts")
	}
}
