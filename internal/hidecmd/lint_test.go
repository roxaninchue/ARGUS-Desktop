package hidecmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestEveryCommandHidden fails when code that can run on Windows starts a program without
// hiding its console window. A command counts as hidden when one of the hiding helpers is
// applied within a few lines of exec.Command.
func TestEveryCommandHidden(t *testing.T) {
	root, _ := filepath.Abs("../..")
	hiders := []string{"hidecmd.Hide(", "Hide(", "hideWindow(", "hideConsole(", "hide(", "quiet(", "killTree(", "HideBrowserWindow(", "SysProcAttr", "detached("}
	// Programs that are GUI apps (no console) or only run on macOS/Linux.
	allowed := []string{`"explorer"`, `"rundll32"`, `"open"`, `"xdg-open"`, `"pkexec"`, `"hdiutil"`, `"ditto"`, `"xattr"`, `"osascript"`, `"crontab"`, `"tesseract"`, `lookCmd("brew")`, `"taskkill"`, `"/usr/libexec/java_home"`, `"/usr/bin/xcode-select"`}
	skipName := func(n string) bool {
		for _, s := range []string{"_test.go", "_darwin.go", "_linux.go", "_unix.go", "_other.go"} {
			if strings.HasSuffix(n, s) {
				return true
			}
		}
		return false
	}
	var bad []string
	_ = filepath.Walk(root, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		if fi.IsDir() {
			if rel == "cloud" || rel == "ci" || rel == "frontend" || rel == "node_modules" || strings.HasPrefix(fi.Name(), ".") || fi.Name() == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") || skipName(fi.Name()) {
			return nil
		}
		b, _ := os.ReadFile(p)
		src := string(b)
		if strings.Contains(src, "//go:build darwin") || strings.Contains(src, "//go:build linux") || strings.Contains(src, "//go:build !windows") {
			return nil
		}
		lines := strings.Split(src, "\n")
		for i, l := range lines {
			if !strings.Contains(l, "exec.Command") || strings.HasPrefix(strings.TrimSpace(l), "//") {
				continue
			}
			win := strings.Join(lines[i:min(len(lines), i+4)], "\n")
			ok := false
			for _, a := range allowed {
				if strings.Contains(l, a) {
					ok = true
				}
			}
			for _, h := range hiders {
				if strings.Contains(win, h) {
					ok = true
				}
			}
			if !ok {
				bad = append(bad, rel+":"+itoa(i+1)+": "+strings.TrimSpace(l))
			}
		}
		return nil
	})
	if len(bad) > 0 {
		t.Fatalf("commands that would flash a console window on Windows (wrap them in hidecmd.Hide):\n%s", strings.Join(bad, "\n"))
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}
