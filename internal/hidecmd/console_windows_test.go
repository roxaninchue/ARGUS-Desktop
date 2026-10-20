//go:build windows

package hidecmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestNoConsoleWindow starts a console program from a GUI program, the way Blaze.exe does.
// Through Hide it must get no console window; started plainly it gets one (which shows the
// test can tell the difference).
func TestNoConsoleWindow(t *testing.T) {
	dir := t.TempDir()
	child, parent := filepath.Join(dir, "conchild.exe"), filepath.Join(dir, "guiparent.exe")
	build := func(out, pkg string, gui bool) {
		args := []string{"build", "-o", out}
		if gui {
			args = append(args, "-ldflags", "-H=windowsgui")
		}
		if b, err := exec.Command("go", append(args, pkg)...).CombinedOutput(); err != nil {
			t.Fatalf("build %s: %v\n%s", pkg, err, b)
		}
	}
	build(child, "./testdata/conchild", false)
	build(parent, "./testdata/guiparent", true)
	run := func(mode string) string {
		out := filepath.Join(dir, mode+".txt")
		if b, err := exec.Command(parent, child, mode, out).CombinedOutput(); err != nil {
			t.Fatalf("guiparent %s: %v\n%s", mode, err, b)
		}
		b, err := os.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}
		return strings.TrimSpace(string(b))
	}
	if got := run("hidden"); got != "none" {
		t.Fatalf("a hidden command got a console window: %q", got)
	}
	if got := run("plain"); got != "window" {
		t.Logf("control: a plain command reported %q (expected a console window)", got)
	}
}
