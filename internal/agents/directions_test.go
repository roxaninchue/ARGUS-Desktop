package agents

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// A Directions branch starts from the user's uncommitted page, and its own changes
// still apply cleanly back to the folder.
func TestCarryDirtyThenApply(t *testing.T) {
	if !HasGit() {
		t.Skip("no git")
	}
	src := t.TempDir()
	run := func(dir string, args ...string) {
		c := exec.Command("git", append([]string{"-C", dir}, args...)...)
		c.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	run(src, "init", "-q")
	run(src, "config", "core.autocrlf", "false") // LF fixtures on Windows too
	_ = os.WriteFile(filepath.Join(src, "README.md"), []byte("hi\n"), 0o644)
	run(src, "add", "-A")
	run(src, "commit", "-qm", "init")
	// Uncommitted: a new page and an edited file.
	_ = os.WriteFile(filepath.Join(src, "index.html"), []byte("<h1>Draft</h1>\n<p>keep</p>\n"), 0o644)
	_ = os.WriteFile(filepath.Join(src, "README.md"), []byte("hi there\n"), 0o644)

	iso, err := Isolate(src, filepath.Join(t.TempDir(), "w"), "argus/direction-a")
	if err != nil {
		t.Fatal(err)
	}
	defer iso.Remove(true)
	if !iso.Dirty {
		t.Fatal("source should be dirty")
	}
	if err := iso.CarryDirty(); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(iso.Path, "index.html"))
	if err != nil || !strings.Contains(string(b), "Draft") {
		t.Fatalf("draft not carried: %v %q", err, b)
	}
	// Nothing changed yet relative to the new base.
	if cs, _, _ := iso.Changes(0); len(cs) != 0 {
		t.Fatalf("carried state counted as changes: %v", cs)
	}
	// The branch redesigns the page; applying brings only that back.
	_ = os.WriteFile(filepath.Join(iso.Path, "index.html"), []byte("<h1>Bold</h1>\n<p>keep</p>\n"), 0o644)
	if _, err := iso.Apply("direction a"); err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(filepath.Join(src, "index.html"))
	if string(b) != "<h1>Bold</h1>\n<p>keep</p>\n" {
		t.Fatalf("applied %q", b)
	}
	r, _ := os.ReadFile(filepath.Join(src, "README.md"))
	if string(r) != "hi there\n" {
		t.Fatalf("readme %q", r)
	}
}

func TestDirectionResultMessage(t *testing.T) {
	v := View{Handle: DirectionPrefix + "b", Status: Done, Kind: Specialist, Mode: "build", Review: "pending", Result: "Warm"}
	m := ResultMessage(v)
	if !strings.Contains(m, "Do NOT apply or discard") || strings.Contains(m, "apply_specialist apply or discard") {
		t.Fatal(m)
	}
	if !IsDirection("direction-a") || IsDirection("api-tests") {
		t.Fatal("IsDirection")
	}
}
