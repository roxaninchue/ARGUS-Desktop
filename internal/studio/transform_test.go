package studio

import "testing"

func TestNormalizeTransform(t *testing.T) {
	cases := []struct {
		in   map[string]string
		want map[string]string
	}{
		{map[string]string{"translate": "12.6px 40.2px"}, map[string]string{"translate": "13px 40px"}},
		{map[string]string{"translate": "-0.4px 0.3px"}, map[string]string{"translate": ""}},
		{map[string]string{"translate": "99999px -5px"}, map[string]string{"translate": "20000px -5px"}},
		{map[string]string{"translate": "8px"}, map[string]string{"translate": "8px 0px"}},
		{map[string]string{"width": "319.99px", "height": "0.2px"}, map[string]string{"width": "320px", "height": "1px"}},
		{map[string]string{"width": "50.04%"}, map[string]string{"width": "50%"}},
		{map[string]string{"width": "auto"}, map[string]string{"width": "auto"}},
		{map[string]string{"scale": "1.0400"}, map[string]string{"scale": "1.04"}},
		{map[string]string{"scale": "1.001"}, map[string]string{"scale": ""}},
		{map[string]string{"scale": "0.001"}, map[string]string{"scale": "0.05"}},
		{map[string]string{"rotate": "370deg"}, map[string]string{"rotate": "10deg"}},
		{map[string]string{"rotate": "-190"}, map[string]string{"rotate": "170deg"}},
		{map[string]string{"translate": "none"}, map[string]string{"translate": ""}},
	}
	for _, c := range cases {
		got, err := NormalizeTransform(c.in)
		if err != nil {
			t.Fatalf("%v: %v", c.in, err)
		}
		if len(got) != len(c.want) {
			t.Fatalf("%v: got %v want %v", c.in, got, c.want)
		}
		for k, v := range c.want {
			if got[k] != v {
				t.Errorf("%v: %s = %q, want %q", c.in, k, got[k], v)
			}
		}
	}
}

func TestNormalizeTransformRejects(t *testing.T) {
	for _, in := range []map[string]string{
		{"translate": "left top"},
		{"width": "wide"},
		{"width": "140%"},
		{"scale": "big"},
		{"color": "red"},
	} {
		if _, err := NormalizeTransform(in); err == nil {
			t.Errorf("%v: want an error", in)
		}
	}
}

func TestIsTransform(t *testing.T) {
	if !IsTransform(map[string]string{"translate": "1px 2px", "width": "3px"}) {
		t.Error("move+size is a transform")
	}
	if IsTransform(map[string]string{"translate": "1px 2px", "color": "#fff"}) {
		t.Error("a colour isn't")
	}
	if IsTransform(nil) {
		t.Error("nothing isn't")
	}
}

func TestNormHex(t *testing.T) {
	cases := map[string]string{
		"#ABC":                    "#aabbcc",
		"abc":                     "#aabbcc",
		"#4DA8FF":                 "#4da8ff",
		"#4da8ffff":               "#4da8ff",
		"#4da8ff80":               "#4da8ff80",
		"#abcd":                   "#aabbccdd",
		"rgb(77, 168, 255)":       "#4da8ff",
		"rgba(77, 168, 255, 0.5)": "#4da8ff80",
		"rgba(0,0,0,0)":           "#00000000",
		"rgb(255 201 77 / 50%)":   "#ffc94d80",
	}
	for in, want := range cases {
		got, ok := NormHex(in)
		if !ok || got != want {
			t.Errorf("NormHex(%q) = %q,%v want %q", in, got, ok, want)
		}
	}
	for _, bad := range []string{"", "#12", "#12345", "#ggg", "blue-ish", "#1234567"} {
		if _, ok := NormHex(bad); ok {
			t.Errorf("NormHex(%q) should fail", bad)
		}
	}
}

func TestNormalizeStyle(t *testing.T) {
	got, err := NormalizeStyle(map[string]string{"color": "RGB(255, 0, 0)", "background-color": "#FFC94D", "translate": "4.4px 9.6px", "font-size": "18px"})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"color": "#ff0000", "background-color": "#ffc94d", "translate": "4px 10px", "font-size": "18px"}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, want %q", k, got[k], v)
		}
	}
	if _, err := NormalizeStyle(map[string]string{"width": "lots"}); err == nil {
		t.Error("a bad width should fail")
	}
}

// A move written back lands in the element's style attribute, and a reset removes it.
func TestApplyMoveAndReset(t *testing.T) {
	src := `<section><h1 class="hero">Hi</h1></section>`
	props, _ := NormalizeTransform(map[string]string{"translate": "10.2px 20.7px", "width": "300.4px"})
	out, err := Apply(src, LangHTML, Edit{Kind: "style", Target: Target{Tag: "h1", Classes: []string{"hero"}, Text: "Hi"}, Props: props})
	if err != nil {
		t.Fatal(err)
	}
	if out != `<section><h1 class="hero" style="translate: 10px 21px; width: 300px">Hi</h1></section>` {
		t.Fatalf("move: %s", out)
	}
	props, _ = NormalizeTransform(map[string]string{"translate": "0px 0px"})
	back, err := Apply(out, LangHTML, Edit{Kind: "style", Target: Target{Tag: "h1", Classes: []string{"hero"}, Text: "Hi"}, Props: props})
	if err != nil {
		t.Fatal(err)
	}
	if back != `<section><h1 class="hero" style="width: 300px">Hi</h1></section>` {
		t.Fatalf("reset: %s", back)
	}
}
