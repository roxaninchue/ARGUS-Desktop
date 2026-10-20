package main

import "testing"

func TestReleaseFromSums(t *testing.T) {
	sums := []byte("f83f2dc207d762b400745db941feeb70fb4884691ed4236b2006b242b223d367  Blaze-v0.19.1-linux-amd64.tar.gz\n" +
		"07c20cc087a4f7de24a3a844f96774bdee83e5d671b966270deff462faebfb62  Blaze-v0.19.1-macos-universal.dmg\n" +
		"9699c4b0b580558774e1055c404b7fe02945e068c34d9bdba2d0593ab6ef18e2  Blaze-v0.19.1-windows-amd64-portable.exe\n" +
		"22ac458bbd6cd0c8db6e68c9cc06e09fe5f664030e3c3f560664f9bc502a7da9  blaze-desktop-0.19.1-1-x86_64.pkg.tar.zst\n" +
		"c09d750f77f4b2d0bf852fec2d27750f5213b747bc44381b2b5e092cecb9a8cd  blaze-desktop_0.19.1_amd64.deb\n" +
		"bad line\n" +
		"c09d750f77f4b2d0bf852fec2d27750f5213b747bc44381b2b5e092cecb9a8cd  ../escape.deb\n")
	rel, err := releaseFromSums("https://dl.example/dl/desktop/", sums)
	if err != nil {
		t.Fatal(err)
	}
	if rel.Tag != "v0.19.1" {
		t.Fatalf("tag %q", rel.Tag)
	}
	if len(rel.Assets) != 7 { // 5 files + SHA256SUMS + .sig
		t.Fatalf("assets %d", len(rel.Assets))
	}
	if got := rel.Assets[0].Download; got != "https://dl.example/dl/desktop/v0.19.1/Blaze-v0.19.1-linux-amd64.tar.gz" {
		t.Fatalf("url %q", got)
	}
	if rel.Assets[6].Name != "SHA256SUMS.sig" || rel.HTMLURL != updatePage {
		t.Fatalf("sig/page %+v", rel)
	}
	if _, err := releaseFromSums("x", []byte("nothing here")); err == nil {
		t.Fatal("expected an error for an empty list")
	}
}
