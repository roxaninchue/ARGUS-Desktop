package cloudsync

import (
	"fmt"
	"strings"
	"time"

	"github.com/lacksonkapapahue/argus-desktop/internal/mind"
	"github.com/lacksonkapapahue/argus-desktop/internal/store"
)

// MemLine is one memory line as it syncs (the phone's MemoryStore.Line). Updated is ms.
type MemLine struct {
	Kind, Subject, Text, Related string
	Updated                      int64
}

// MemoryStore is where memory lines live on this device.
type MemoryStore interface {
	All() []MemLine
	Add(l MemLine) error
	// Remove deletes every line whose text has l's MemoryID.
	Remove(l MemLine) error
}

// MemoryAdapter is kind "memory": one item per line, id MemoryID(text), so the same fact typed
// on two devices is one item (connect/SyncAdapters.kt MemoryAdapter).
type MemoryAdapter struct {
	Store MemoryStore

	cache map[string]MemLine // id → line, from Stamps until the next Apply/Flush (one round)
}

func (m *MemoryAdapter) Kind() string { return KindMemory }

func (m *MemoryAdapter) byID() map[string]MemLine {
	if m.cache != nil {
		return m.cache
	}
	out := map[string]MemLine{}
	for _, l := range m.Store.All() {
		if strings.TrimSpace(l.Text) == "" {
			continue
		}
		out[MemoryID(l.Text)] = l
	}
	return out
}

func memStamp(l MemLine) string { return fmt.Sprintf("%d:%s:%s", l.Updated, l.Kind, l.Subject) }

func (m *MemoryAdapter) Stamps() map[string]string {
	out := map[string]string{}
	m.cache = nil
	m.cache = m.byID()
	for id, l := range m.cache {
		out[id] = memStamp(l)
	}
	return out
}

func (m *MemoryAdapter) Stamp(id string) (string, bool) {
	l, ok := m.byID()[id]
	if !ok {
		return "", false
	}
	return memStamp(l), true
}

func (m *MemoryAdapter) Read(id string) (Item, bool) {
	l, ok := m.byID()[id]
	if !ok {
		return Item{}, false
	}
	return MemoryItem(l.Kind, l.Subject, l.Text, l.Related, l.Updated), true
}

func (m *MemoryAdapter) Apply(it Item) error {
	m.cache = nil
	here, ok := m.byID()[it.ID]
	if it.Deleted() {
		if ok {
			return m.Store.Remove(here)
		}
		return nil
	}
	text := optString(it.Data, "text")
	if strings.TrimSpace(text) == "" || MemoryID(text) != it.ID {
		return nil // a broken item: never store a line under another line's id
	}
	kind, subject := optString(it.Data, "kind"), optString(it.Data, "subject")
	if ok && here.Kind == kind && here.Subject == subject {
		return nil
	}
	if ok {
		if err := m.Store.Remove(here); err != nil {
			return err
		}
	}
	return m.Store.Add(MemLine{Kind: kind, Subject: subject, Text: text, Related: optString(it.Data, "related"), Updated: it.T})
}

func (m *MemoryAdapter) Shrink(Item, func(Item) bool) (Item, bool) { return Item{}, false }
func (m *MemoryAdapter) PushDeletes() bool                         { return true }
func (m *MemoryAdapter) Flush() error                              { m.cache = nil; return nil }

// ---------------- Desktop: mind.db (+ the argus.db copy Argus CLI reads) ----------------

// MindStore is Desktop's memory: mind.db memories (kind/subject/text/related, the phone's
// shape) is the source of truth, and every line is kept in step with an argus.db row (the
// table Argus CLI reads) through memories.mirror, as mind_app.go mindSave does. Lines from
// other devices are inserted exactly (mind.Insert), never merged into a similar line here.
// Argus is nil = no argus.db copy.
type MindStore struct {
	Mind  *mind.DB
	Argus *store.DB
}

func (s *MindStore) All() []MemLine {
	var out []MemLine
	for _, m := range s.Mind.All() {
		out = append(out, MemLine{Kind: m.Kind, Subject: m.Subject, Text: m.Text, Related: m.Related, Updated: m.Updated * 1000})
	}
	return out
}

func (s *MindStore) Add(l MemLine) error {
	// Exact insert (no dedupe; see mind.Insert), keeping the line's own time.
	id, err := s.Mind.Insert(mind.Memory{Kind: l.Kind, Subject: l.Subject, Text: l.Text, Related: l.Related, Updated: l.Updated / 1000})
	if err != nil || s.Argus == nil {
		return err
	}
	cur, ok := s.Mind.Get(id)
	if !ok {
		return nil
	}
	at := time.UnixMilli(l.Updated)
	if l.Updated <= 0 {
		at = time.Now()
	}
	bid, _, err := s.Argus.Add(cur.Text, "", "", at)
	if err != nil {
		return err
	}
	if bid > 0 {
		s.Mind.SetMirror(id, bid)
		_ = s.Argus.SetCategory(bid, cur.Kind)
	}
	return nil
}

func (s *MindStore) Remove(l MemLine) error {
	want := MemoryID(l.Text)
	for _, m := range s.Mind.All() {
		if MemoryID(m.Text) != want {
			continue
		}
		if err := s.Mind.Delete(m.ID); err != nil {
			return err
		}
		if s.Argus != nil && m.Mirror != 0 {
			_ = s.Argus.Delete(m.Mirror)
		}
	}
	if s.Argus != nil { // an argus.db line with the same text but no mirror link (CLI-typed)
		for _, b := range s.Argus.Memories() {
			if MemoryID(b.Text) == want {
				_ = s.Argus.Delete(b.ID)
			}
		}
	}
	return nil
}

// ---------------- Argus CLI alone: argus.db only ----------------

// ArgusDBStore syncs argus.db memories directly (for Argus CLI on a PC without Desktop).
// argus.db has no subject column: kind comes from category ("" = fact), subject is "".
// Never run it next to MindStore on the same PC: Desktop's MindStore already covers argus.db.
type ArgusDBStore struct{ DB *store.DB }

func (s *ArgusDBStore) All() []MemLine {
	var out []MemLine
	for _, m := range s.DB.Memories() {
		out = append(out, MemLine{Kind: mind.NormKind(m.Category), Text: m.Text, Updated: m.Updated.UnixMilli()})
	}
	return out
}

func (s *ArgusDBStore) Add(l MemLine) error {
	at := time.UnixMilli(l.Updated)
	if l.Updated <= 0 {
		at = time.Now()
	}
	id, _, err := s.DB.Add(l.Text, "", "", at)
	if err == nil && id > 0 && l.Kind != "" {
		_ = s.DB.SetCategory(id, mind.NormKind(l.Kind))
	}
	return err
}

func (s *ArgusDBStore) Remove(l MemLine) error {
	want := MemoryID(l.Text)
	for _, m := range s.DB.Memories() {
		if MemoryID(m.Text) == want {
			if err := s.DB.Delete(m.ID); err != nil {
				return err
			}
		}
	}
	return nil
}
