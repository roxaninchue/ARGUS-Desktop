package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// v0.19.15: the picked folder can be a symlink. pick returns (picked link, real folder, outside folder).
func pick(t *testing.T) (string, string, string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on Windows")
	}
	base := t.TempDir()
	real, out := filepath.Join(base, "data", "work"), filepath.Join(base, "elsewhere")
	for _, d := range []string{real, out} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	link := filepath.Join(base, "home", "work")
	_ = os.MkdirAll(filepath.Dir(link), 0o755)
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	return link, real, out
}

type card struct{ title, detail string }

func pickedEnv(t *testing.T, root, cwd string, yes bool) (*Env, *[]card) {
	var cards []card
	e := &Env{Root: root, Shell: NewShell("sh", "/bin/sh", cwd), Confirm: func(title, detail string) bool {
		cards = append(cards, card{title, detail})
		return yes
	}}
	return e, &cards
}

func jargs(kv map[string]any) string { b, _ := json.Marshal(kv); return string(b) }

func TestPickedSymlinkRootInside(t *testing.T) {
	link, real, _ := pick(t)
	if err := os.WriteFile(filepath.Join(real, "notes.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{
		filepath.Join(link, "notes.txt"),           // as picked
		filepath.Join(real, "notes.txt"),           // as the shell's pwd -P writes it
		filepath.Join(real, "new", "report.md"),    // not there yet
		filepath.Join(link, "new", "deep", "x.md"), // not there yet, under the link
		link, real,
	} {
		if !inside(link, p) {
			t.Errorf("inside(%s, %s) = false, want true", link, p)
		}
	}
	if !inside(real, filepath.Join(link, "notes.txt")) {
		t.Error("the link's files are inside the real folder too")
	}
	// read + write + edit a file inside the symlinked picked folder: no card, the work is done.
	for _, cwd := range []string{link, real} {
		e, cards := pickedEnv(t, link, cwd, false)
		if r := Run(e, "file_read", jargs(map[string]any{"path": "notes.txt"})); !strings.Contains(r, "hello") {
			t.Fatalf("read via %s: %s", cwd, r)
		}
		w := filepath.Join(real, "out.md")
		if r := Run(e, "file_write", jargs(map[string]any{"path": w, "content": "# report\n"})); !strings.HasPrefix(r, "Wrote") {
			t.Fatalf("write via %s: %s", cwd, r)
		}
		if r := Run(e, "file_edit", jargs(map[string]any{"path": "out.md", "old_text": "# report", "new_text": "# Report"})); !strings.HasPrefix(r, "Edited") {
			t.Fatalf("edit via %s: %s", cwd, r)
		}
		if b, _ := os.ReadFile(filepath.Join(link, "out.md")); string(b) != "# Report\n" {
			t.Fatalf("file = %q", b)
		}
		if len(*cards) != 0 {
			t.Fatalf("asked inside the picked folder (cwd %s): %+v", cwd, *cards)
		}
	}
}

func TestSymlinkOutOfPickedFolderStillAsks(t *testing.T) {
	link, real, out := pick(t)
	if err := os.Symlink(out, filepath.Join(real, "escape")); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{filepath.Join(link, "escape", "x.txt"), filepath.Join(real, "escape", "x.txt"), filepath.Join(link, "escape"), filepath.Join(out, "x.txt"), filepath.Join(link, "..", "elsewhere", "x.txt")} {
		if inside(link, p) {
			t.Errorf("inside(%s, %s) = true: a symlink out of the picked folder must not count as inside", link, p)
		}
	}
	e, cards := pickedEnv(t, link, link, false)
	r := Run(e, "file_write", jargs(map[string]any{"path": "escape/x.txt", "content": "nope"}))
	if !strings.Contains(r, "did NOT approve") || len(*cards) != 1 {
		t.Fatalf("write through the escape link: %q cards=%+v", r, *cards)
	}
	if _, err := os.Stat(filepath.Join(out, "x.txt")); err == nil {
		t.Fatal("wrote outside the picked folder without a yes")
	}
	if err := os.WriteFile(filepath.Join(out, "y.txt"), []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r = Run(e, "file_edit", jargs(map[string]any{"path": "escape/y.txt", "old_text": "a", "new_text": "b"}))
	if !strings.Contains(r, "did NOT approve") || len(*cards) != 2 {
		t.Fatalf("edit through the escape link: %q cards=%+v", r, *cards)
	}
	if b, _ := os.ReadFile(filepath.Join(out, "y.txt")); string(b) != "a\n" {
		t.Fatalf("edited outside without a yes: %q", b)
	}
}

func TestCardShowsPickedPath(t *testing.T) {
	link, real, out := pick(t)
	if err := os.WriteFile(filepath.Join(real, "old.log"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	e, cards := pickedEnv(t, link, real, false) // the shell sits in the real folder (pwd -P)
	_ = Run(e, "file_delete", jargs(map[string]any{"path": "old.log"}))
	if len(*cards) != 1 || (*cards)[0].detail != filepath.Join(link, "old.log") {
		t.Fatalf("delete card = %+v, want the path under the picked folder %s", *cards, filepath.Join(link, "old.log"))
	}
	if strings.Contains((*cards)[0].detail, real) {
		t.Fatalf("card shows where the symlink leads: %q", (*cards)[0].detail)
	}
	// Shown: picked form for inside paths, as-is for everything else
	if got := e.Shown(filepath.Join(real, "a", "b.txt")); got != filepath.Join(link, "a", "b.txt") {
		t.Errorf("Shown(real/a/b.txt) = %q", got)
	}
	if got := e.Shown(real); got != link {
		t.Errorf("Shown(real) = %q, want %q", got, link)
	}
	if got := e.Shown(filepath.Join(link, "c.txt")); got != filepath.Join(link, "c.txt") {
		t.Errorf("Shown(link/c.txt) = %q", got)
	}
	if got := e.Shown(filepath.Join(out, "z.txt")); got != filepath.Join(out, "z.txt") {
		t.Errorf("Shown(outside) = %q", got)
	}
	// an outside write still shows the real outside path
	_ = Run(e, "file_write", jargs(map[string]any{"path": filepath.Join(out, "z.txt"), "content": "z"}))
	if len(*cards) != 2 || (*cards)[1].detail != filepath.Join(out, "z.txt") || !strings.Contains((*cards)[1].title, "outside") {
		t.Fatalf("outside write card = %+v", *cards)
	}
}
