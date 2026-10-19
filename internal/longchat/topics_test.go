package longchat

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/lacksonkapapahue/blaze-desktop/internal/llm"
	"github.com/lacksonkapapahue/blaze-desktop/internal/topics"
)

// A longchat.db made by v0.18 (no topic tables) opens, keeps its index and summary, and
// gains the topic tables; the old tables are unchanged.
func TestOldDBLoadsAndGainsTopics(t *testing.T) {
	p := filepath.Join(t.TempDir(), "longchat.db")
	old, err := sql.Open("sqlite", "file:"+filepath.ToSlash(p))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := old.Exec(schema); err != nil { // the v0.18 schema only
		t.Fatal(err)
	}
	if _, err := old.Exec(`INSERT INTO chat_fts(chat_id, turn_id, role, content) VALUES ('main', 3, 'user', 'the pizza dough rests overnight');
		INSERT INTO chat_state(chat_id, summary, upto, indexed) VALUES ('main', 'Goal: pizza', 2, 4);`); err != nil {
		t.Fatal(err)
	}
	old.Close()

	d, err := Open(p)
	if err != nil {
		t.Fatalf("old db must open: %v", err)
	}
	if d.Indexed("main") != 4 || d.State("main").Summary != "Goal: pizza" || len(d.Search("main", "pizza", 5)) != 1 {
		t.Fatal("old data lost")
	}
	// No topic row yet: a fresh state, never an error.
	if st := d.Topics("main"); st.Active != 0 || len(st.Topics) != 0 || st.TurnTopic == nil {
		t.Fatalf("fresh state expected, got %+v", st)
	}
	h := []llm.Message{{Role: "user", Content: "I want to make a pizza tonight"}, {Role: "assistant", Content: "Great, start the dough now."}}
	st := topics.New()
	st.Segment(h, topics.SegOpts{})
	if err := d.SetTopics("main", st); err != nil {
		t.Fatal(err)
	}
	back := d.Topics("main")
	if back.Active != st.Active || back.Get(st.Active).Title != "Pizza" || back.Assigned != 2 {
		t.Fatalf("round trip: %+v", back)
	}
	// Forget (the v0.18 call) leaves topics alone; ForgetTopics drops them.
	d.Forget("main")
	if d.Topics("main").Active == 0 {
		t.Fatal("Forget must not touch topic state")
	}
	d.ForgetTopics("main")
	if d.Topics("main").Active != 0 {
		t.Fatal("ForgetTopics should drop the state")
	}
	if d.MetaValue("topics") != "" {
		t.Fatal("meta starts empty")
	}
	_ = d.SetMetaValue("topics", "off")
	if d.MetaValue("topics") != "off" {
		t.Fatal("meta round trip")
	}
	d.Close()
	// What an older build does with the new file: its schema statements are all IF NOT EXISTS.
	s, _ := sql.Open("sqlite", "file:"+filepath.ToSlash(p))
	if _, err := s.Exec(schema); err != nil {
		t.Fatalf("v0.18 schema on the v0.19 file: %v", err)
	}
	s.Close()
	d2, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	if d2.MetaValue("topics") != "off" {
		t.Fatal("reopen keeps the new tables")
	}
	d2.Close()
}
