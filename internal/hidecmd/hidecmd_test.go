package hidecmd

import (
	"os/exec"
	"testing"
)

func TestHideReturnsSameCmd(t *testing.T) {
	c := exec.Command("git", "--version")
	if Hide(c) != c {
		t.Fatal("Hide must return its argument")
	}
	if Hide(nil) != nil {
		t.Fatal("Hide(nil) must be nil")
	}
}
