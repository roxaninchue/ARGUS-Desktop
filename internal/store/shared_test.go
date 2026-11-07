package store

// Sharing argus.db with Argus CLI: both apps creating it at the same moment, and a
// database written by a newer Argus.

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// rawOpen opens the file the way another program (the CLI) would: its own
// connection, no help from this package.
func rawOpen(t *testing.T, p string) *sql.DB {
	t.Helper()
	s, err := sql.Open("sqlite", "file:"+filepath.ToSlash(p)+"?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatal(err)
	}
	s.SetMaxOpenConns(1)
	t.Cleanup(func() { s.Close() })
	return s
}

func fileSum(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// Several openers (each its own connection, like Desktop and the CLI starting
// together) create the same new file at once: every one succeeds and the schema
// is applied exactly once.
func TestConcurrentFirstRunInit(t *testing.T) {
	for round := 0; round < 5; round++ {
		p := filepath.Join(t.TempDir(), "argus.db")
		const n = 6
		var wg sync.WaitGroup
		start := make(chan struct{})
		errs := make(chan error, n)
		dbs := make(chan *DB, n)
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				d, err := Open(p)
				if err != nil {
					errs <- err
					return
				}
				dbs <- d
			}()
		}
		close(start)
		wg.Wait()
		close(errs)
		close(dbs)
		for err := range errs {
			t.Fatalf("round %d: %v", round, err)
		}
		var first *DB
		for d := range dbs {
			if first == nil {
				first = d
			} else {
				d.Close()
			}
		}
		if v := first.Version(); v != SchemaVersion() {
			t.Fatalf("user_version %d, want %d", v, SchemaVersion())
		}
		if _, _, err := first.Add(fmt.Sprintf("round %d fact", round), "", "", time.Time{}); err != nil {
			t.Fatal(err)
		}
		first.Close()
	}
}

// The other app migrates while we wait for the write lock: Open must re-read
// user_version under the lock and skip the migration instead of failing with
// "table memories already exists".
func TestMigrateRechecksVersionUnderLock(t *testing.T) {
	p := filepath.Join(t.TempDir(), "argus.db")
	cli := rawOpen(t, p)
	if _, err := cli.Exec(`PRAGMA journal_mode=WAL`); err != nil {
		t.Fatal(err)
	}
	conn, err := cli.Conn(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(t.Context(), `BEGIN IMMEDIATE`); err != nil {
		t.Fatal(err)
	}

	type res struct {
		d   *DB
		err error
	}
	done := make(chan res, 1)
	go func() {
		d, err := Open(p) // reads user_version 0, then waits for the lock
		done <- res{d, err}
	}()
	time.Sleep(300 * time.Millisecond)
	select {
	case r := <-done:
		t.Fatalf("Open returned while the CLI held the write lock: %v", r.err)
	default:
	}
	// The CLI finishes its own migration and lets go.
	if _, err := conn.ExecContext(t.Context(), migrations[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(t.Context(), fmt.Sprintf(`PRAGMA user_version = %d`, len(migrations))); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(t.Context(), `COMMIT`); err != nil {
		t.Fatal(err)
	}

	r := <-done
	if r.err != nil {
		t.Fatalf("Open after the CLI migrated: %v", r.err)
	}
	defer r.d.Close()
	if r.d.Version() != SchemaVersion() {
		t.Fatal(r.d.Version())
	}
	if _, _, err := r.d.Add("Uses pnpm", "", "", time.Time{}); err != nil {
		t.Fatal(err)
	}
}

// A database from a newer Argus is refused with ErrNewerSchema and left exactly
// as it was.
func TestNewerSchemaRefused(t *testing.T) {
	p := filepath.Join(t.TempDir(), "argus.db")
	d, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	_, _, _ = d.Add("Keep me safe", "", "", time.Time{})
	// A future Argus moves the schema on.
	if _, err := d.sql.Exec(fmt.Sprintf(`CREATE TABLE future(x); PRAGMA user_version = %d`, SchemaVersion()+1)); err != nil {
		t.Fatal(err)
	}
	d.Close() // checkpoints the WAL into the main file
	before := fileSum(t, p)

	_, err = Open(p)
	if !errors.Is(err, ErrNewerSchema) {
		t.Fatalf("got %v, want ErrNewerSchema", err)
	}
	if !strings.Contains(err.Error(), "update Argus") {
		t.Fatalf("error should ask to update Argus: %v", err)
	}
	if fileSum(t, p) != before {
		t.Fatal("refusing must not change the database")
	}
	s := rawOpen(t, p)
	var v int
	var txt string
	_ = s.QueryRow(`PRAGMA user_version`).Scan(&v)
	_ = s.QueryRow(`SELECT text FROM memories`).Scan(&txt)
	if v != SchemaVersion()+1 || txt != "Keep me safe" {
		t.Fatal(v, txt)
	}
}

// A file that claims the current schema but lacks its tables is refused.
func TestBrokenSchemaRefused(t *testing.T) {
	p := filepath.Join(t.TempDir(), "argus.db")
	s := rawOpen(t, p)
	if _, err := s.Exec(`CREATE TABLE memories(id INTEGER PRIMARY KEY, other TEXT); PRAGMA user_version = 1`); err != nil {
		t.Fatal(err)
	}
	s.Close()
	if _, err := Open(p); err == nil || !strings.Contains(err.Error(), "missing table") {
		t.Fatalf("got %v", err)
	}
}
