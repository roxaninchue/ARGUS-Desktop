package sysinfo

import (
	"runtime"
	"testing"
)

func TestMacStubOnlyOnDarwin(t *testing.T) {
	if runtime.GOOS != "darwin" && MacStub("/usr/bin/git") {
		t.Fatal("MacStub must be false off macOS")
	}
	if MacStub("/opt/homebrew/bin/git") {
		t.Fatal("a real install is never a stub")
	}
}
