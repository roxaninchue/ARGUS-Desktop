package tools

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lacksonkapapahue/argus-desktop/internal/store"
)

// An argus.db from a newer Argus: Desktop runs without memory, leaves the file
// alone, and says "please update Argus" instead of a vague database error.
func TestNewerSchemaRunsWithoutMemory(t *testing.T) {
	home := t.TempDir()
	testHome(t, home)
	p := filepath.Join(home, "argus.db")
	s, err := sql.Open("sqlite", "file:"+filepath.ToSlash(p))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Exec(fmt.Sprintf(`CREATE TABLE future(x); PRAGMA user_version = %d`, store.SchemaVersion()+1)); err != nil {
		t.Fatal(err)
	}
	s.Close()
	before, _ := os.ReadFile(p)

	if DB() != nil {
		t.Fatal("a newer schema must not be opened")
	}
	if n := UpdateNotice(); !strings.Contains(n, "update Argus") {
		t.Fatalf("notice %q", n)
	}
	if r := Run(&Env{}, "remember", `{"fact":"Uses pnpm"}`); !strings.Contains(r, "update Argus") {
		t.Fatalf("remember said %q", r)
	}
	if r := Run(&Env{}, "recall", `{"query":"pnpm"}`); !strings.Contains(r, "update Argus") {
		t.Fatalf("recall said %q", r)
	}
	if Memory() != "" || MemoryFor("pnpm") != "" {
		t.Fatal("no memory should be served")
	}
	after, _ := os.ReadFile(p)
	if string(before) != string(after) {
		t.Fatal("argus.db changed")
	}
}
