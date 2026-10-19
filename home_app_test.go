package main

import "testing"

func TestHomeLocale(t *testing.T) {
	for in, want := range map[string][2]string{"en-ZM": {"en", "ZM"}, "en_US.UTF-8": {"en", "US"}, "fr": {"fr", ""}, "": {"en", ""}, "C": {"en", ""}, "zh-Hant-TW": {"zh", "TW"}} {
		l, c := splitLocale(in)
		if l != want[0] || c != want[1] {
			t.Errorf("splitLocale(%q) = %q %q", in, l, c)
		}
	}
	if baseName("/home/luke/HomeGlow/") != "HomeGlow" || baseName(`C:\work\site`) != "site" {
		t.Error("baseName")
	}
}
