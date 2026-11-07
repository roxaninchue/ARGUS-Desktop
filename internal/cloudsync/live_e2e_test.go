package cloudsync

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lacksonkapapahue/argus-desktop/internal/mind"
	"github.com/lacksonkapapahue/argus-desktop/internal/store"
)

// TestLiveE2E runs Desktop's real sync stack (Engine + MindStore + ConvFiles, 600k PBKDF2) against
// the live gateway, one step at a time, with an independent Python "phone" in between
// (work/e2e_sync_phone.py in the Desktop project). Skipped unless ARGUS_SYNC_E2E_DIR is set.
//
//	ARGUS_SYNC_E2E_DIR    the Desktop home kept across steps (mind.db, argus.db, chats, state)
//	ARGUS_SYNC_E2E_TOKEN  an Argus session (bz_…) of a throwaway QA account
//	ARGUS_SYNC_E2E_PASS   the sync passphrase
//	ARGUS_SYNC_E2E_STEP   A (set up + 5 memories + 5 messages up), C (take the phone's 5+5, delete
//	                      2+1 memories and a chat), E (take the phone's deletes)
//	ARGUS_SYNC_E2E_BASE   gateway root (default https://blaze-gateway.blaze-app.workers.dev)
func TestLiveE2E(t *testing.T) {
	dir := os.Getenv("ARGUS_SYNC_E2E_DIR")
	if dir == "" {
		t.Skip("set ARGUS_SYNC_E2E_DIR (and _TOKEN, _PASS, _STEP) for the live run")
	}
	tok, pass, step := os.Getenv("ARGUS_SYNC_E2E_TOKEN"), os.Getenv("ARGUS_SYNC_E2E_PASS"), os.Getenv("ARGUS_SYNC_E2E_STEP")
	base := os.Getenv("ARGUS_SYNC_E2E_BASE")
	if base == "" {
		base = "https://blaze-gateway.blaze-app.workers.dev"
	}
	run := strings.TrimSpace(readFile(t, filepath.Join(dir, "run_id")))
	if run == "" {
		t.Fatal("no run_id file")
	}
	chatID, delChat := "e2e"+run, "e2edel"+run
	c := &Client{Base: base, Token: tok, UserAgent: "Argus-Desktop/e2e"}
	ctx := context.Background()

	_ = os.MkdirAll(filepath.Join(dir, "desktop", "chats"), 0o700)
	md, err := mind.Open(filepath.Join(dir, "desktop", "mind.db"))
	if err != nil {
		t.Fatal(err)
	}
	bd, err := store.Open(filepath.Join(dir, "argus.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer md.Close()
	defer bd.Close()
	ms := &MindStore{Mind: md, Argus: bd}
	conv := &ConvFiles{Dir: filepath.Join(dir, "desktop", "chats"), AliasPath: filepath.Join(dir, "cloudsync", "desktop-alias.json")}
	st := LoadState(filepath.Join(dir, "cloudsync", "desktop-state.json"))

	keyPath := filepath.Join(dir, "key.hex") // the app keeps this in the OS keychain
	var key []byte
	if step == "A" {
		t0 := time.Now()
		key, err = Enable(ctx, c, pass, 0)
		if err != nil {
			t.Fatal("enable:", err)
		}
		t.Logf("enable (600k PBKDF2 + set up) took %v", time.Since(t0).Round(time.Millisecond))
		if err := os.WriteFile(keyPath, []byte(hex.EncodeToString(key)), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := Enable(ctx, c, "not the passphrase at all", 0); err != ErrBadPassphrase {
			t.Fatalf("a wrong passphrase on a second device: %v", err)
		}
	} else {
		key, _ = hex.DecodeString(strings.TrimSpace(readFile(t, keyPath)))
	}
	eng := &Engine{API: c, Key: key, State: st, Window: 300, BackfillPushes: 5,
		Adapters: append([]Adapter{&MemoryAdapter{Store: ms}}, ChatAdapters(conv, st)...)}
	ev := map[string]any{"step": step, "run": run, "at": time.Now().UTC().Format(time.RFC3339)}
	round := func(name string) Report {
		r, err := eng.Run(ctx)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		ev[name] = map[string]any{"pulled": r.Pulled, "pushed": r.Pushed, "merged": r.Merged, "failed_open": r.FailedOpen, "backlog": r.Backlog, "cursor": st.Cursor, "epoch": st.Epoch}
		t.Logf("%s: %+v cursor=%d", name, r, st.Cursor)
		return r
	}
	memText := func(i int) string {
		return fmt.Sprintf("E2E-%s desktop fact %d: likes route %d/%c & \"quotes\" <ok>", run, i, i, 'a'+i)
	}

	switch step {
	case "A":
		var ids []string
		for i := 1; i <= 5; i++ {
			l := MemLine{Kind: "fact", Subject: "e2e", Text: memText(i), Updated: time.Now().UnixMilli()}
			if err := ms.Add(l); err != nil {
				t.Fatal(err)
			}
			ids = append(ids, MemoryID(l.Text))
		}
		msgs := []map[string]any{}
		for i := 1; i <= 5; i++ {
			role := "user"
			if i%2 == 0 {
				role = "assistant"
			}
			msgs = append(msgs, map[string]any{"role": role, "content": fmt.Sprintf("E2E-%s desktop message %d (%s)", run, i, role)})
			if i == 2 {
				msgs = append(msgs, map[string]any{"role": "assistant", "content": "", "tool_calls": []any{map[string]any{"id": "c1"}}},
					map[string]any{"role": "tool", "content": "E2E-" + run + " tool output stays local", "tool_call_id": "c1"})
			}
		}
		writeConvFile(t, conv.Dir, chatID, "E2E chat "+run, msgs)
		writeConvFile(t, conv.Dir, delChat, "E2E chat to delete "+run, []map[string]any{{"role": "user", "content": "E2E-" + run + " in a chat that gets deleted"}})
		writeConvFile(t, conv.Dir, "private-e2e"+run, "Private", []map[string]any{{"role": "user", "content": "E2E-" + run + " PRIVATE never syncs"}})
		r := round("round1")
		if r.Pushed != 5+5+1+2+0 { // 5 memory lines, 5 messages + 1 in delChat, 2 chat heads
			t.Logf("note: pushed %d (expected 13)", r.Pushed)
		}
		var msgIDs []string
		mm, _ := conv.Messages(chatID)
		for _, m := range mm {
			id, _ := MsgItemID(chatID, m.GID)
			msgIDs = append(msgIDs, id)
		}
		if len(msgIDs) != 5 {
			t.Fatalf("desktop has %d syncable messages, want 5", len(msgIDs))
		}
		ev["memory_ids"], ev["memory_texts"], ev["msg_ids"], ev["chat"], ev["del_chat"] = ids, []string{memText(1), memText(2), memText(3), memText(4), memText(5)}, msgIDs, chatID, delChat
		if r2 := round("round2_idle"); r2.Pushed != 0 {
			t.Fatalf("an idle round pushed %d", r2.Pushed)
		}
	case "C":
		ph := readJSON(t, filepath.Join(dir, "phone-B.json"))
		round("round3_take_phone")
		got := 0
		for _, x := range ph["memory_texts"].([]any) {
			text := x.(string)
			if m, ok := mindHas(md, text); ok && m.Mirror != 0 && argusHas(bd, text) {
				got++
			} else {
				t.Errorf("phone memory missing in mind.db/argus.db: %q", text)
			}
		}
		ev["phone_memories_landed"] = got
		raw := rawConv(t, conv.Dir, chatID)
		landed := 0
		for _, x := range ph["msg_texts"].([]any) {
			for _, m := range raw {
				if m["content"] == x.(string) {
					landed++
					break
				}
			}
		}
		ev["phone_messages_landed"] = landed
		if got != 5 || landed != 5 {
			t.Fatalf("phone → desktop: %d/5 memories, %d/5 messages", got, landed)
		}
		if r := round("round4_idle"); r.Pushed != 0 {
			t.Fatalf("taking the phone's items echoed %d back", r.Pushed)
		}
		// Deletes here: 2 desktop lines, 1 phone line, and a whole chat.
		var gone []string
		for _, text := range []string{memText(1), memText(2), ph["memory_texts"].([]any)[0].(string)} {
			m, ok := mindHas(md, text)
			if !ok {
				t.Fatalf("no line %q", text)
			}
			if err := md.Delete(m.ID); err != nil {
				t.Fatal(err)
			}
			if m.Mirror != 0 {
				_ = bd.Delete(m.Mirror)
			}
			gone = append(gone, MemoryID(text))
		}
		if err := os.Remove(filepath.Join(conv.Dir, delChat+".json")); err != nil {
			t.Fatal(err)
		}
		r := round("round5_deletes")
		if r.Pushed != 4 {
			t.Fatalf("deletes pushed %d items, want 4 tombstones", r.Pushed)
		}
		for _, id := range gone {
			if e, _ := st.Get(KindMemory, id); !e.Deleted {
				t.Fatalf("%s not recorded as a pushed tombstone", id)
			}
		}
		ev["deleted_memory_ids"], ev["deleted_chat"] = gone, delChat
	case "E":
		ph := readJSON(t, filepath.Join(dir, "phone-D.json"))
		round("round6_take_phone_deletes")
		n := 0
		for _, x := range ph["deleted_memory_texts"].([]any) {
			text := x.(string)
			if _, ok := mindHas(md, text); ok || argusHas(bd, text) {
				t.Errorf("phone-deleted line still here: %q", text)
			} else {
				n++
			}
		}
		ev["phone_deletes_applied"] = n
		if n != 2 {
			t.Fatalf("phone deletes applied %d/2", n)
		}
		if r := round("round7_idle"); r.Pushed != 0 {
			t.Fatalf("idle round pushed %d", r.Pushed)
		}
		left := 0
		for _, m := range md.All() {
			if strings.HasPrefix(m.Text, "E2E-"+run) {
				left++
			}
		}
		ev["e2e_lines_left"] = left // 5 + 5 - 3 - 2 = 5
	default:
		t.Fatalf("unknown step %q", step)
	}
	b, _ := json.MarshalIndent(ev, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "desktop-"+step+".json"), b, 0o600); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func readJSON(t *testing.T, p string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(readFile(t, p)), &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func writeConvFile(t *testing.T, dir, id, title string, msgs []map[string]any) {
	t.Helper()
	c := map[string]any{"id": id, "project": "", "title": title, "folder": "", "updated": time.Now().UTC().Format(time.RFC3339Nano), "messages": msgs}
	b, _ := json.Marshal(c)
	if err := os.WriteFile(filepath.Join(dir, id+".json"), b, 0o600); err != nil {
		t.Fatal(err)
	}
}

func rawConv(t *testing.T, dir, id string) []map[string]any {
	t.Helper()
	var c struct {
		Messages []map[string]any `json:"messages"`
	}
	_ = json.Unmarshal([]byte(readFile(t, filepath.Join(dir, id+".json"))), &c)
	return c.Messages
}

func mindHas(md *mind.DB, text string) (mind.Memory, bool) {
	for _, m := range md.All() {
		if m.Text == text {
			return m, true
		}
	}
	return mind.Memory{}, false
}

func argusHas(bd *store.DB, text string) bool {
	for _, m := range bd.Memories() {
		if m.Text == text {
			return true
		}
	}
	return false
}
