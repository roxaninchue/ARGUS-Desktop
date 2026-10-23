package cloudsync

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/lacksonkapapahue/blaze-desktop/internal/mind"
	"github.com/lacksonkapapahue/blaze-desktop/internal/store"
)

const testPass = "correct horse battery staple"

// ---------------- an in-memory "phone" ----------------

type memMemory struct{ lines []MemLine }

func (m *memMemory) All() []MemLine { return append([]MemLine(nil), m.lines...) }
func (m *memMemory) Add(l MemLine) error {
	m.lines = append(m.lines, l)
	return nil
}
func (m *memMemory) Remove(l MemLine) error {
	var keep []MemLine
	for _, x := range m.lines {
		if MemoryID(x.Text) != MemoryID(l.Text) {
			keep = append(keep, x)
		}
	}
	m.lines = keep
	return nil
}
func (m *memMemory) has(text string) bool {
	for _, x := range m.lines {
		if x.Text == text {
			return true
		}
	}
	return false
}

type memChat struct {
	head ChatHead
	msgs []ChatMsg
}

type memChats struct{ chats map[string]*memChat }

func newMemChats() *memChats { return &memChats{chats: map[string]*memChat{}} }
func (m *memChats) Chats() []string {
	var out []string
	for id := range m.chats {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}
func (m *memChats) Head(id string) (ChatHead, bool) {
	c, ok := m.chats[id]
	if !ok {
		return ChatHead{}, false
	}
	return c.head, true
}
func (m *memChats) Messages(id string) ([]ChatMsg, bool) {
	c, ok := m.chats[id]
	if !ok {
		return nil, false
	}
	return append([]ChatMsg(nil), c.msgs...), true
}
func (m *memChats) get(id string) *memChat {
	c, ok := m.chats[id]
	if !ok {
		c = &memChat{head: ChatHead{ID: id}}
		m.chats[id] = c
	}
	return c
}
func (m *memChats) PutHead(h ChatHead) error { m.get(h.ID).head = h; return nil }
func (m *memChats) PutMsg(chat string, msg ChatMsg) error {
	c := m.get(chat)
	for i, x := range c.msgs {
		if x.GID == msg.GID {
			c.msgs[i] = msg
			return nil
		}
	}
	c.msgs = append(c.msgs, msg)
	return nil
}
func (m *memChats) RemoveMsg(chat, gid string) error {
	c, ok := m.chats[chat]
	if !ok {
		return nil
	}
	var keep []ChatMsg
	for _, x := range c.msgs {
		if x.GID != gid {
			keep = append(keep, x)
		}
	}
	c.msgs = keep
	return nil
}
func (m *memChats) Delete(id string) error { delete(m.chats, id); return nil }

// ---------------- devices ----------------

type phone struct {
	mem   *memMemory
	chats *memChats
	eng   *Engine
}

func newPhone(g *fakeGateway, key []byte) *phone {
	p := &phone{mem: &memMemory{}, chats: newMemChats()}
	st := NewState()
	p.eng = &Engine{API: g.client(), Key: key, State: st, Sleep: func(time.Duration) {},
		Adapters: append([]Adapter{&MemoryAdapter{Store: p.mem}}, ChatAdapters(p.chats, st)...)}
	return p
}

type desktop struct {
	dir   string
	mind  *mind.DB
	blaze *store.DB
	conv  *ConvFiles
	ms    *MindStore
	eng   *Engine
}

func newDesktop(t *testing.T, g *fakeGateway, key []byte) *desktop {
	t.Helper()
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "desktop", "chats"), 0o700)
	md, err := mind.Open(filepath.Join(dir, "desktop", "mind.db"))
	if err != nil {
		t.Fatal(err)
	}
	bd, err := store.Open(filepath.Join(dir, "blaze.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { md.Close(); bd.Close() })
	d := &desktop{dir: dir, mind: md, blaze: bd}
	d.conv = &ConvFiles{Dir: filepath.Join(dir, "desktop", "chats"), AliasPath: filepath.Join(dir, "cloudsync", "gids.json")}
	_ = os.MkdirAll(d.conv.Dir, 0o700)
	d.ms = &MindStore{Mind: md, Blaze: bd}
	st := LoadState(filepath.Join(dir, "cloudsync", "state.json"))
	d.eng = &Engine{API: g.client(), Key: key, State: st, Sleep: func(time.Duration) {},
		Adapters: append([]Adapter{&MemoryAdapter{Store: d.ms}}, ChatAdapters(d.conv, st)...)}
	return d
}

func (d *desktop) blazeHas(text string) bool {
	for _, m := range d.blaze.Memories() {
		if m.Text == text {
			return true
		}
	}
	return false
}

func (d *desktop) mindHas(text string) (mind.Memory, bool) {
	for _, m := range d.mind.All() {
		if m.Text == text {
			return m, true
		}
	}
	return mind.Memory{}, false
}

// writeConv writes a chat file the way the app does (Conv with llm.Message, no gid/at).
func (d *desktop) writeConv(t *testing.T, id string, msgs []map[string]any) {
	t.Helper()
	c := map[string]any{"id": id, "project": "", "title": "Chat", "folder": "", "updated": time.Now().UTC().Format(time.RFC3339Nano), "messages": msgs}
	b, _ := json.Marshal(c)
	if err := os.WriteFile(filepath.Join(d.conv.Dir, id+".json"), b, 0o600); err != nil {
		t.Fatal(err)
	}
}

// resave simulates the app re-saving a chat from []llm.Message: anything but role/content/tool
// fields is dropped (gid, at).
func (d *desktop) resave(t *testing.T, id string) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(d.conv.Dir, id+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var c struct {
		Messages []struct {
			Role       string `json:"role"`
			Content    string `json:"content"`
			ToolCallID string `json:"tool_call_id,omitempty"`
			ToolCalls  []any  `json:"tool_calls,omitempty"`
		} `json:"messages"`
	}
	_ = json.Unmarshal(b, &c)
	var msgs []map[string]any
	for _, m := range c.Messages {
		x := map[string]any{"role": m.Role, "content": m.Content}
		if m.ToolCallID != "" {
			x["tool_call_id"] = m.ToolCallID
		}
		if len(m.ToolCalls) > 0 {
			x["tool_calls"] = m.ToolCalls
		}
		msgs = append(msgs, x)
	}
	d.writeConv(t, id, msgs)
}

func (d *desktop) rawMessages(t *testing.T, id string) []map[string]any {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(d.conv.Dir, id+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var c struct {
		Messages []map[string]any `json:"messages"`
	}
	_ = json.Unmarshal(b, &c)
	return c.Messages
}

func run(t *testing.T, e *Engine) Report {
	t.Helper()
	r, err := e.Run(context.Background())
	if err != nil {
		t.Fatalf("round: %v", err)
	}
	return r
}

// setUp makes an account copy on the fake gateway (the phone's Enable) and returns the key.
func setUp(t *testing.T, g *fakeGateway) []byte {
	t.Helper()
	key, err := Enable(context.Background(), g.client(), testPass, 10_000)
	if err != nil {
		t.Fatal(err)
	}
	// A second device unlocks with the same passphrase and gets the same key.
	key2, err := Enable(context.Background(), g.client(), testPass, 10_000)
	if err != nil || hex.EncodeToString(key2) != hex.EncodeToString(key) {
		t.Fatalf("second device: %v", err)
	}
	if _, err := Enable(context.Background(), g.client(), "not the passphrase", 10_000); err != ErrBadPassphrase {
		t.Fatalf("wrong passphrase: %v", err)
	}
	return key
}

// ---------------- golden vectors ----------------

type vectorFile struct {
	Passphrase, Salt, KDF string
	KeyHex                string `json:"key_hex"`
	Vectors               []struct {
		Name, Kind, ID, Plain, Blob string
	}
}

// TestMobileGoldenVectors opens blobs sealed per Blaze Android SyncCrypto.kt with Android
// org.json envelopes (testdata/mobile_vectors.json, made by an independent implementation of
// the Kotlin spec, work/gen_mobile_vectors.py), plus the CLI's vectors from a straight JCE port.
func TestMobileGoldenVectors(t *testing.T) {
	b, err := os.ReadFile("testdata/mobile_vectors.json")
	if err != nil {
		t.Fatal(err)
	}
	var vf vectorFile
	if err := json.Unmarshal(b, &vf); err != nil {
		t.Fatal(err)
	}
	key, err := DeriveKey(vf.Passphrase, vf.Salt, IterationsOf(vf.KDF))
	if err != nil || hex.EncodeToString(key) != vf.KeyHex {
		t.Fatalf("key %x %v", key, err)
	}
	// The CLI's JCE-made vectors (same passphrase and salt).
	const cliBlob = "yiI4YrB3Pps5orM0tG+WaFkOWUvIB+IEuk9B34RHjE8fqImgqvaxD3R1P/q+CMGJwQuCKd76/xhYJ7e0e1AjM2znlIiOsvy9V78q3J09KwpUlJeZ6dvrsdE37xBbDN3LcKASFqu0Z8gdRT79mSp6jGWFWmbgTgX7KyOcHH6+vinzMjwzt6845prgy01nva0h"
	const cliCheck = "bfs0h78StNNI4dMdKaEdbaKTv/Nab03kw3Nndjmq7xwbLM5HWf6Co1C8RVRxOphsXGBIUEPGo9MzxwrsRZLqnRyexg=="
	if s, err := Open(key, "memory", "m123", cliBlob); err != nil || !strings.Contains(s, "Prefers metric units") {
		t.Fatalf("CLI vector: %v %q", err, s)
	}
	if !Checks(key, cliCheck) {
		t.Fatal("CLI check vector")
	}
	for _, v := range vf.Vectors {
		plain, err := Open(key, v.Kind, v.ID, v.Blob)
		if err != nil || plain != v.Plain {
			t.Fatalf("%s: %v %q", v.Name, err, plain)
		}
		if v.Kind == "_check" {
			if !Checks(key, v.Blob) {
				t.Fatal("check blob")
			}
			continue
		}
		it, err := ParseItem(v.Kind, v.ID, plain)
		if err != nil {
			t.Fatalf("%s: %v", v.Name, err)
		}
		switch v.Name {
		case "memory":
			if MemoryID(optString(it.Data, "text")) != v.ID || optString(it.Data, "subject") != "home" || it.T != 1761130000123 {
				t.Fatalf("memory %+v", it)
			}
		case "memory_tombstone", "chat_tombstone":
			if !it.Deleted() {
				t.Fatalf("%s not a tombstone", v.Name)
			}
		case "msg_user":
			// Android escapes '/' as "\/": it must decode to a plain slash.
			if optString(it.Data, "text") != `Remind me: the dentist is on Friday / 10:00 "sharp"` || optLong(it.Data, "at") != 1761130001000 {
				t.Fatalf("msg %+v", it.Data)
			}
		case "msg_blaze":
			if optString(it.Data, "role") != RoleBlaze || !strings.Contains(optString(it.Data, "text"), "✓") {
				t.Fatalf("msg %+v", it.Data)
			}
		}
		// What Desktop seals for the same item opens to an envelope the phone's parser accepts.
		blob, err := Seal(key, it.Kind, it.ID, it.Envelope())
		if err != nil {
			t.Fatal(err)
		}
		back, err := Open(key, it.Kind, it.ID, blob)
		if err != nil {
			t.Fatal(err)
		}
		again, err := ParseItem(it.Kind, it.ID, back)
		if err != nil || canon(again.Data) != canon(it.Data) || again.T != it.T || again.Deleted() != it.Deleted() {
			t.Fatalf("%s re-seal: %v", v.Name, err)
		}
		if !strings.HasPrefix(back, `{"v":1,"kind":`) {
			t.Fatalf("envelope shape %q", back)
		}
	}
	// Phone and Desktop derive the same memory id for the same fact.
	if MemoryID("Lives in São Paulo; works at 9:00!") != "mc8226c78a007e648e5aca5b0eb815fba" {
		t.Fatal("memory id differs from Android")
	}
	// Phone and Desktop derive the same legacy message id (Gid.legacy).
	if g := LegacyGID("USER", 0, "hi"); !regexp.MustCompile(`^l[0-9a-f]{16}$`).MatchString(g) {
		t.Fatal(g)
	}
}

// TestExportVectorsForMobile writes Desktop-sealed blobs for the reverse check (a phone or the
// Python spec implementation opens them) when DSYNC_EXPORT names a file.
func TestExportVectorsForMobile(t *testing.T) {
	out := os.Getenv("DSYNC_EXPORT")
	if out == "" {
		t.Skip("DSYNC_EXPORT not set")
	}
	key, _ := hex.DecodeString("ef177144eec9420cbc1093d2a8b344a92bc506d0d4ec9c028dd19f8324d8c1e6")
	items := []Item{
		MemoryItem("profile", "home", "Lives in São Paulo; works at 9:00!", "work", 1761130000123),
		{Kind: KindMessage, ID: "main:l0123456789abcdef", T: 1, Data: msgData(ChatMsg{GID: "l0123456789abcdef", Role: RoleBlaze, Text: "Noted </b> & done ✓"})},
		{Kind: KindChat, ID: "c-old", T: 5},
	}
	var vs []map[string]string
	for _, it := range items {
		b, _ := Seal(key, it.Kind, it.ID, it.Envelope())
		vs = append(vs, map[string]string{"kind": it.Kind, "id": it.ID, "plain": it.Envelope(), "blob": b})
	}
	c, _ := MakeCheck(key)
	vs = append(vs, map[string]string{"kind": "_check", "id": "v1", "plain": "blaze-sync-check-v1", "blob": c})
	b, _ := json.MarshalIndent(vs, "", " ")
	if err := os.WriteFile(out, b, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestSealRoundTripAndBinding(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	text := `{"v":1,"data":{"text":"Hello ✓"}}` + strings.Repeat("x", 4000)
	b1, err := Seal(key, "msg", "main:a", text)
	if err != nil {
		t.Fatal(err)
	}
	b2, _ := Seal(key, "msg", "main:a", text)
	if b1 == b2 {
		t.Fatal("nonce reused")
	}
	if len(b1) >= len(text) {
		t.Fatal("not gzipped")
	}
	if s, err := Open(key, "msg", "main:a", b1); err != nil || s != text {
		t.Fatal("round trip")
	}
	for _, kid := range [][2]string{{"msg", "main:b"}, {"memory", "main:a"}} {
		if _, err := Open(key, kid[0], kid[1], b1); err == nil {
			t.Fatalf("opened under %v", kid)
		}
	}
	raw, _ := base64.StdEncoding.DecodeString(b1)
	raw[len(raw)-1] ^= 1
	if _, err := Open(key, "msg", "main:a", base64.StdEncoding.EncodeToString(raw)); err == nil {
		t.Fatal("tampered blob opened")
	}
	other := append([]byte(nil), key...)
	other[0] ^= 1
	if _, err := Open(other, "msg", "main:a", b1); err == nil {
		t.Fatal("wrong key opened")
	}
}

// ---------------- end to end against the fake gateway ----------------

func TestMemoryBothWaysDeletesAndCiphertextOnly(t *testing.T) {
	g := newFakeGateway(t)
	key := setUp(t, g)
	ph := newPhone(g, key)
	dk := newDesktop(t, g, key)

	phoneFact := "My dentist is Dr Banda at Levy Mall, Fridays secret-token-PHONE"
	ph.mem.Add(MemLine{Kind: "person", Subject: "dentist", Text: phoneFact, Updated: time.Now().UnixMilli()})
	run(t, ph.eng)
	run(t, dk.eng)
	m, ok := dk.mindHas(phoneFact)
	if !ok || m.Kind != "person" || m.Subject != "dentist" {
		t.Fatalf("Desktop mind.db missing the phone's fact: %+v", dk.mind.All())
	}
	if !dk.blazeHas(phoneFact) || m.Mirror == 0 {
		t.Fatal("blaze.db (Blaze CLI) missing the phone's fact")
	}

	deskFact := "Prefers replies in British English secret-token-DESKTOP"
	if err := dk.ms.Add(MemLine{Kind: "preference", Subject: "language", Text: deskFact}); err != nil {
		t.Fatal(err)
	}
	run(t, dk.eng)
	run(t, ph.eng)
	if !ph.mem.has(deskFact) {
		t.Fatal("phone missing the Desktop fact")
	}

	// Deletes: phone → Desktop (mind.db and blaze.db), then Desktop → phone.
	ph.mem.Remove(MemLine{Text: phoneFact})
	run(t, ph.eng)
	run(t, dk.eng)
	if _, ok := dk.mindHas(phoneFact); ok || dk.blazeHas(phoneFact) {
		t.Fatal("phone delete did not reach Desktop / blaze.db")
	}
	dm, _ := dk.mindHas(deskFact)
	_ = dk.ms.Remove(MemLine{Text: dm.Text})
	r := run(t, dk.eng)
	if r.Pushed != 1 {
		t.Fatalf("tombstone pushes: %d", r.Pushed)
	}
	if r2 := run(t, dk.eng); r2.Pushed != 0 {
		t.Fatal("tombstone pushed twice")
	}
	run(t, ph.eng)
	if ph.mem.has(deskFact) || len(ph.mem.lines) != 0 {
		t.Fatalf("Desktop delete did not reach the phone: %+v", ph.mem.lines)
	}
	if g.live(KindMemory) != 0 {
		t.Fatal("server still has live memory items")
	}

	// The server only ever saw ciphertext: no plaintext, passphrase or key in any request body.
	all := g.allBodies()
	for _, secret := range []string{"secret-token-PHONE", "secret-token-DESKTOP", "Dr Banda", "British English", "dentist", "language",
		testPass, hex.EncodeToString(key), base64.StdEncoding.EncodeToString(key), "blaze-sync-check-v1"} {
		if strings.Contains(all, secret) {
			t.Fatalf("server saw %q", secret)
		}
	}
	// Every blob pushed is one this key opens, bound to its kind/id.
	n := 0
	for _, b := range g.bodies {
		var p struct {
			Items []Out `json:"items"`
		}
		if json.Unmarshal([]byte(b), &p) != nil {
			continue
		}
		for _, it := range p.Items {
			if it.Deleted {
				if it.Blob != "" {
					t.Fatal("tombstone with a blob")
				}
				continue
			}
			if _, err := Open(key, it.Kind, it.ID, it.Blob); err != nil {
				t.Fatalf("pushed blob doesn't open: %v", err)
			}
			n++
		}
	}
	if n < 2 {
		t.Fatalf("expected pushed blobs, got %d", n)
	}
}

func TestIdempotentRePull(t *testing.T) {
	g := newFakeGateway(t)
	key := setUp(t, g)
	ph := newPhone(g, key)
	for i, f := range []string{"Allergic to penicillin", "Lives in Lusaka", "Works at Northrise"} {
		ph.mem.Add(MemLine{Kind: "profile", Subject: "me", Text: f, Updated: int64(1000 + i)})
	}
	ph.chats.PutHead(ChatHead{ID: "main", Title: "Chat", Updated: 5000})
	ph.chats.PutMsg("main", ChatMsg{GID: "abcdefgh-k1", Role: RoleUser, Text: "hello", At: 5000})
	run(t, ph.eng)

	dk := newDesktop(t, g, key)
	run(t, dk.eng)
	count := func() (int, int, int) {
		ms, _ := dk.conv.Messages("main")
		return len(dk.mind.All()), len(dk.blaze.Memories()), len(ms)
	}
	a, b, c := count()
	if a != 3 || b != 3 || c != 1 {
		t.Fatalf("first pull: %d %d %d", a, b, c)
	}
	if r := run(t, dk.eng); r.Pulled != 0 || r.Pushed != 0 {
		t.Fatalf("second round did something: %+v", r)
	}
	// Cursor rewound: everything comes again and changes nothing.
	dk.eng.State.Cursor = 0
	if r := run(t, dk.eng); r.Pushed != 0 {
		t.Fatalf("re-pull pushed: %+v", r)
	}
	// A lost state file: a full re-pull over the same data neither duplicates nor pushes.
	_ = os.Remove(filepath.Join(dk.dir, "cloudsync", "state.json"))
	dk.eng.State = LoadState(filepath.Join(dk.dir, "cloudsync", "state.json"))
	dk.eng.Adapters = append([]Adapter{&MemoryAdapter{Store: dk.ms}}, ChatAdapters(dk.conv, dk.eng.State)...)
	r := run(t, dk.eng)
	if x, y, z := count(); x != 3 || y != 3 || z != 1 {
		t.Fatalf("fresh state duplicated: %d %d %d", x, y, z)
	}
	if r.Pushed != 0 {
		t.Fatalf("fresh state pushed %d", r.Pushed)
	}
	// The state file survives a restart.
	if st := LoadState(filepath.Join(dk.dir, "cloudsync", "state.json")); st.Cursor == 0 || len(st.Items) == 0 {
		t.Fatal("state not saved")
	}
}

func TestConflictLastWriterWins(t *testing.T) {
	g := newFakeGateway(t)
	key := setUp(t, g)
	a, b := newPhone(g, key), newPhone(g, key)
	text := "Favourite food is nshima with kapenta"
	a.mem.Add(MemLine{Kind: "preference", Subject: "food", Text: text, Updated: 1000})
	run(t, a.eng)
	run(t, b.eng)

	// Both change the same line; B's change is newer and wins everywhere, whichever syncs first.
	a.mem.lines[0].Subject, a.mem.lines[0].Updated = "meals", 5000
	b.mem.lines[0].Subject, b.mem.lines[0].Updated = "dinner", 6000
	run(t, a.eng)
	run(t, b.eng)
	run(t, a.eng)
	if a.mem.lines[0].Subject != "dinner" || b.mem.lines[0].Subject != "dinner" {
		t.Fatalf("newer change lost: a=%q b=%q", a.mem.lines[0].Subject, b.mem.lines[0].Subject)
	}

	// The older change loses even when it syncs last.
	a.mem.lines[0].Subject, a.mem.lines[0].Updated = "lunch", 9000
	b.mem.lines[0].Subject, b.mem.lines[0].Updated = "snack", 8000
	run(t, a.eng)
	run(t, b.eng)
	if b.mem.lines[0].Subject != "lunch" {
		t.Fatalf("older change won: %q", b.mem.lines[0].Subject)
	}

	// Server-side conflict: another device writes between B's pull and B's push (stale base_rev).
	b.mem.lines[0].Subject, b.mem.lines[0].Updated = "brunch", 20000
	other := MemoryItem("preference", "supper", text, "", 30000)
	blob, _ := Seal(key, KindMemory, other.ID, other.Envelope())
	k := KindMemory + "\x00" + other.ID
	first := true
	hook := func() {
		if !first {
			return
		}
		first = false
		row := g.items[k]
		g.seq++
		g.items[k] = &fakeItem{Kind: KindMemory, ID: other.ID, Seq: g.seq, Rev: row.Rev + 1, Blob: blob, UpdatedAt: 1}
	}
	g.mu.Lock()
	g.beforePush = hook
	g.mu.Unlock()
	run(t, b.eng)
	if b.mem.lines[0].Subject != "supper" {
		t.Fatalf("server conflict not resolved to the newer copy: %q", b.mem.lines[0].Subject)
	}
	run(t, a.eng)
	if a.mem.lines[0].Subject != "supper" {
		t.Fatalf("a: %q", a.mem.lines[0].Subject)
	}
}

func TestMessagesBothWaysNoEcho(t *testing.T) {
	g := newFakeGateway(t)
	key := setUp(t, g)
	ph := newPhone(g, key)
	dk := newDesktop(t, g, key)
	dk.writeConv(t, "main", []map[string]any{
		{"role": "user", "content": "What's on Friday?"},
		{"role": "assistant", "content": "", "tool_calls": []any{map[string]any{"id": "t1"}}},
		{"role": "tool", "content": "calendar: dentist 10:00", "tool_call_id": "t1"},
		{"role": "assistant", "content": "The dentist at 10:00."},
	})
	run(t, dk.eng)
	run(t, ph.eng)
	pm, _ := ph.chats.Messages("main")
	if len(pm) != 2 || pm[0].Role != RoleUser || pm[1].Role != RoleBlaze || pm[1].Text != "The dentist at 10:00." {
		t.Fatalf("phone got %+v", pm)
	}
	if strings.Contains(g.allBodies(), "dentist") || strings.Contains(g.allBodies(), "calendar") {
		t.Fatal("chat text reached the server")
	}

	// The phone answers: the message lands in Desktop's chat file as a user message.
	ph.chats.PutMsg("main", ChatMsg{GID: "k3x9a0b1-lz8q4k2m1", Role: RoleUser, Text: "Remind me Thursday night", At: time.Now().UnixMilli()})
	ph.chats.PutMsg("main", ChatMsg{GID: "k3x9a0b1-lz8q4k2m2", Role: "STEP", Text: "thinking…", At: time.Now().UnixMilli()})
	run(t, ph.eng)
	run(t, dk.eng)
	raw := dk.rawMessages(t, "main")
	last := raw[len(raw)-1]
	if last["role"] != "user" || last["content"] != "Remind me Thursday night" || len(raw) != 5 {
		t.Fatalf("Desktop chat: %+v", raw)
	}

	// The app re-saves the chat (gid/at dropped): nothing is sent back as a new message.
	dk.resave(t, "main")
	if r := run(t, dk.eng); r.Pushed != 0 {
		t.Fatalf("re-save echoed %d item(s)", r.Pushed)
	}
	run(t, ph.eng)
	if pm, _ := ph.chats.Messages("main"); len(pm) != 4 { // 2 from Desktop + the phone's 2
		t.Fatalf("phone has %d messages", len(pm))
	}

	// Desktop replies: one new message to the phone, with the phone's legacy gid rule.
	raw = dk.rawMessages(t, "main")
	raw = append(raw, map[string]any{"role": "assistant", "content": "I'll remind you Thursday at 20:00."})
	dk.writeConv(t, "main", raw)
	if r := run(t, dk.eng); r.Pushed != 1 {
		t.Fatalf("reply pushes: %+v", r)
	}
	run(t, ph.eng)
	pm, _ = ph.chats.Messages("main")
	got := pm[len(pm)-1]
	if got.Text != "I'll remind you Thursday at 20:00." || got.GID != LegacyGID(RoleBlaze, 0, got.Text) {
		t.Fatalf("phone got %+v", got)
	}
}

func TestChatDeletePropagates(t *testing.T) {
	g := newFakeGateway(t)
	key := setUp(t, g)
	ph := newPhone(g, key)
	dk := newDesktop(t, g, key)
	ph.chats.PutHead(ChatHead{ID: "c-trip", Title: "Trip", Updated: 1000})
	ph.chats.PutMsg("c-trip", ChatMsg{GID: "abcdefgh-1", Role: RoleUser, Text: "Book Livingstone", At: 1000})
	run(t, ph.eng)
	run(t, dk.eng)
	if h, ok := dk.conv.Head("c-trip"); !ok || h.Title != "Trip" {
		t.Fatal("chat did not arrive")
	}
	ph.chats.Delete("c-trip")
	run(t, ph.eng)
	run(t, dk.eng)
	if _, ok := dk.conv.Head("c-trip"); ok {
		t.Fatal("chat delete did not reach Desktop")
	}
	// Deleted on Desktop: the phone loses it too, and a late message doesn't bring it back.
	dk.writeConv(t, "c-two", []map[string]any{{"role": "user", "content": "x"}})
	run(t, dk.eng)
	run(t, ph.eng)
	if _, ok := ph.chats.Head("c-two"); !ok {
		t.Fatal("c-two missing on phone")
	}
	_ = os.Remove(filepath.Join(dk.conv.Dir, "c-two.json"))
	run(t, dk.eng)
	run(t, ph.eng)
	if _, ok := ph.chats.Head("c-two"); ok {
		t.Fatal("Desktop delete did not reach the phone")
	}
	// The main conversation is never deleted by sync.
	dk.writeConv(t, "main", []map[string]any{{"role": "user", "content": "keep me"}})
	if err := dk.conv.Delete("main"); err != nil {
		t.Fatal(err)
	}
	if _, ok := dk.conv.Head("main"); !ok {
		t.Fatal("main deleted")
	}
	// Private chats never sync.
	dk.writeConv(t, "private-1", []map[string]any{{"role": "user", "content": "hidden"}})
	run(t, dk.eng)
	if strings.Contains(strings.Join(g.paths, " "), "private") {
		t.Fatal("private chat path")
	}
	for _, id := range dk.conv.Chats() {
		if strings.HasPrefix(id, "private") {
			t.Fatal("private chat listed")
		}
	}
}

func TestBusyRetryPagingAndErrors(t *testing.T) {
	g := newFakeGateway(t)
	key := setUp(t, g)
	ph := newPhone(g, key)
	for i := 0; i < 10; i++ {
		ph.mem.Add(MemLine{Kind: "fact", Subject: "n", Text: "Fact number " + string(rune('A'+i)), Updated: int64(i + 1)})
		// (near-identical lines on purpose: synced lines are inserted exactly, never merged here)
	}
	g.busyOnce = true
	if r := run(t, ph.eng); r.Pushed != 10 {
		t.Fatalf("busy retry: %+v", r)
	}
	g.pullLimit = 3
	dk := newDesktop(t, g, key)
	run(t, dk.eng)
	if len(dk.mind.All()) != 10 {
		t.Fatalf("paged pull got %d", len(dk.mind.All()))
	}
	// Another passphrase: the account's check blob refuses the key.
	other, _ := DeriveKey("a different passphrase", NewSalt(), 10_000)
	stranger := newPhone(g, other)
	if _, err := stranger.eng.Run(context.Background()); err != ErrWrongKey {
		t.Fatalf("wrong key: %v", err)
	}
	// Cloud copy deleted from another device.
	if err := g.client().Wipe(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := dk.eng.Run(context.Background()); err != ErrNotSetUp {
		t.Fatalf("not set up: %v", err)
	}
	// Signed out.
	c := &Client{Base: g.srv.URL}
	if _, err := c.Meta(context.Background()); err == nil {
		t.Fatal("no token accepted")
	}
}

func TestResolveRules(t *testing.T) {
	mk := func(kind string, tm int64, d map[string]any) Item { return Item{Kind: kind, ID: "x", T: tm, Data: d} }
	if o, _ := Resolve(mk("memory", 2, map[string]any{"a": 1}), mk("memory", 1, map[string]any{"a": 2})); o != KeepLocal {
		t.Fatal("newer local")
	}
	if o, _ := Resolve(mk("memory", 1, nil), mk("memory", 2, map[string]any{"a": 2})); o != TakeRemote {
		t.Fatal("newer remote beats older tombstone")
	}
	if o, _ := Resolve(mk("memory", 3, nil), mk("memory", 2, map[string]any{"a": 2})); o != KeepLocal {
		t.Fatal("newer tombstone wins")
	}
	// Tie: both sides pick the same copy.
	x, y := mk("memory", 5, map[string]any{"s": "a"}), mk("memory", 5, map[string]any{"s": "b"})
	o1, w1 := Resolve(x, y)
	o2, w2 := Resolve(y, x)
	if canon(w1.Data) != canon(w2.Data) || o1 == o2 {
		t.Fatalf("tie not symmetric: %v %v", o1, o2)
	}
	// A message: the fuller copy wins regardless of time.
	short := mk("msg", 9, map[string]any{"role": "BLAZE", "text": "The den"})
	full := mk("msg", 1, map[string]any{"role": "BLAZE", "text": "The dentist at 10:00."})
	if o, _ := Resolve(short, full); o != TakeRemote {
		t.Fatal("fuller remote")
	}
	if o, _ := Resolve(full, short); o != KeepLocal {
		t.Fatal("fuller local")
	}
	// A chat item: newer fields, union of carried messages.
	a := mk("chat", 1, map[string]any{"title": "Old", "updated": int64(1), "messages": []any{map[string]any{"gid": "g1", "role": "USER", "text": "a", "at": int64(1)}}})
	b := mk("chat", 2, map[string]any{"title": "New", "updated": int64(2), "messages": []any{map[string]any{"gid": "g2", "role": "USER", "text": "b", "at": int64(2)}}})
	o, m := Resolve(a, b)
	if o != Merged || optString(m.Data, "title") != "New" || len(m.Data["messages"].([]any)) != 2 {
		t.Fatalf("chat merge %v %+v", o, m.Data)
	}
}

// D1 budget (Site, 2026-10-22): a first sync sends memory plus each chat's newest Window
// messages; older history follows newest first, at most BackfillPushes pushes per round.
func TestBackfillWindowCapsFirstSync(t *testing.T) {
	g := newFakeGateway(t)
	key := setUp(t, g)
	ph := newPhone(g, key)
	dk := newDesktop(t, g, key)
	const n = 1500
	var msgs []map[string]any
	for i := 0; i < n; i++ {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		msgs = append(msgs, map[string]any{"role": role, "content": "message number " + strconv.Itoa(i)})
	}
	dk.writeConv(t, "main", msgs)
	if _, err := dk.ms.Mind.Insert(mind.Memory{Kind: "fact", Subject: "home", Text: "Lives near the river"}); err != nil {
		t.Fatal(err)
	}
	pushes := func() int {
		g.mu.Lock()
		defer g.mu.Unlock()
		c := 0
		for _, p := range g.paths {
			if p == "POST /v1/sync/push" {
				c++
			}
		}
		return c
	}
	dk.eng.Window, dk.eng.BackfillPushes = 300, 0
	r := run(t, dk.eng)
	if r.Pushed != 300+1+1 || r.Backlog != n-300 { // 300 messages, the chat head, the memory line
		t.Fatalf("first round: pushed %d, backlog %d", r.Pushed, r.Backlog)
	}
	run(t, ph.eng)
	pm, _ := ph.chats.Messages("main")
	if len(pm) != 300 || !ph.mem.has("Lives near the river") {
		t.Fatalf("phone has %d messages after the first round", len(pm))
	}
	for _, m := range pm { // the newest 300
		k, _ := strconv.Atoi(strings.TrimPrefix(m.Text, "message number "))
		if k < n-300 {
			t.Fatalf("an old message went first: %q", m.Text)
		}
	}
	before := pushes()
	dk.eng.BackfillPushes = 2
	r = run(t, dk.eng)
	if r.Pushed != 400 || r.Backlog != n-700 || pushes()-before != 2 {
		t.Fatalf("backfill round: pushed %d, backlog %d, %d pushes", r.Pushed, r.Backlog, pushes()-before)
	}
	run(t, ph.eng)
	pm, _ = ph.chats.Messages("main")
	for _, m := range pm {
		k, _ := strconv.Atoi(strings.TrimPrefix(m.Text, "message number "))
		if k < n-700 {
			t.Fatalf("backfill skipped ahead: %q", m.Text)
		}
	}
	dk.eng.BackfillPushes = 0 // a round without backfill sends nothing old
	if r = run(t, dk.eng); r.Pushed != 0 || r.Backlog != n-700 {
		t.Fatalf("quiet round: %+v", r)
	}
	dk.eng.BackfillPushes = 5
	before = pushes()
	if r = run(t, dk.eng); r.Pushed != n-700 || r.Backlog != 0 || pushes()-before != 4 {
		t.Fatalf("last backfill: pushed %d, backlog %d, %d pushes", r.Pushed, r.Backlog, pushes()-before)
	}
	run(t, ph.eng)
	if pm, _ = ph.chats.Messages("main"); len(pm) != n {
		t.Fatalf("phone has %d of %d messages", len(pm), n)
	}
	if r = run(t, dk.eng); r.Pushed != 0 || r.Backlog != 0 {
		t.Fatalf("settled round: %+v", r)
	}
}
