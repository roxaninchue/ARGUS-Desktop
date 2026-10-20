package main

import "testing"

func TestHasOSAsset(t *testing.T) {
	win := []ghAsset{{Name: "Blaze-v0.19.2-windows-amd64-portable.exe"}}
	if !hasOSAsset(win, "windows") || hasOSAsset(win, "darwin") || hasOSAsset(win, "linux") {
		t.Fatal("a Windows-only release is an update only on Windows")
	}
	all := []ghAsset{{Name: "Blaze-v0.18.0-macos-universal.dmg"}, {Name: "blaze-desktop_0.18.0_amd64.deb"}}
	if !hasOSAsset(all, "darwin") || !hasOSAsset(all, "linux") {
		t.Fatal("mac and Linux files not found")
	}
}

func TestPlatformVersion(t *testing.T) {
	j := []byte(`{"desktop":{"version":"0.19.2","notes":"New key","platforms":{"windows":{"version":"0.19.2"},"windows-setup":{"version":"0.18.0"},"mac":{"version":"0.18.0"}}}}`)
	v, notes, ok := platformVersion(j, "setup")
	if !ok || v != "0.18.0" || notes != "New key" {
		t.Fatalf("setup: %q %q %v", v, notes, ok)
	}
	if v, _, _ := platformVersion(j, "portable"); v != "0.19.2" {
		t.Fatalf("portable: %q", v)
	}
	if _, _, ok := platformVersion([]byte("nope"), "dmg"); ok {
		t.Fatal("bad json must be ignored")
	}
}
