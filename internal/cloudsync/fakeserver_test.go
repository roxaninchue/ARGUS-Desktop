package cloudsync

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// fakeGateway is an in-memory copy of Argus Gateway src/sync.ts (0.8.35): meta, push with
// base_rev conflicts and an account-wide seq, pull pages by seq, tombstones, wipe. It records
// every request body so tests can assert the server only ever sees ciphertext.
type fakeGateway struct {
	t     *testing.T
	mu    sync.Mutex
	token string
	meta  *fakeMeta
	items map[string]*fakeItem // kind \0 id
	seq   int64
	bytes int64

	bodies     []string // every request body
	paths      []string
	busyOnce   bool   // next push answers 409 busy
	beforePush func() // runs under the lock before a push is handled (another device writing)
	pullLimit  int    // server page cap (0 = 200)
	srv        *httptest.Server
}

type fakeMeta struct{ Salt, KDF, Check, Epoch string }

type fakeItem struct {
	Kind, ID  string
	Seq, Rev  int64
	Deleted   bool
	Blob      string
	UpdatedAt int64
}

var (
	fKindRE = regexp.MustCompile(`^[a-z][a-z0-9_]{0,31}$`)
	fB64RE  = regexp.MustCompile(`^[A-Za-z0-9+/]+={0,2}$`)
)

func newFakeGateway(t *testing.T) *fakeGateway {
	g := &fakeGateway{t: t, token: "bz_test", items: map[string]*fakeItem{}}
	g.srv = httptest.NewServer(http.HandlerFunc(g.serve))
	t.Cleanup(g.srv.Close)
	return g
}

func (g *fakeGateway) client() *Client { return &Client{Base: g.srv.URL, Token: g.token} }

func (g *fakeGateway) fail(w http.ResponseWriter, code int, err string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, _ = io.WriteString(w, `{"error":{"code":"`+err+`"}}`)
}

func (g *fakeGateway) send(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func (g *fakeGateway) metaBody() map[string]any {
	lim := map[string]any{"account_bytes": 25 << 20, "item_bytes": 512 << 10, "push_items": 200, "pull_items": 200}
	if g.meta == nil {
		return map[string]any{"set_up": false, "limits": lim}
	}
	return map[string]any{"set_up": true, "salt": g.meta.Salt, "kdf": g.meta.KDF, "check": g.meta.Check, "epoch": g.meta.Epoch,
		"seq": g.seq, "bytes": g.bytes, "limits": lim}
}

func wire(it *fakeItem) map[string]any {
	var blob any
	if !it.Deleted {
		blob = it.Blob
	}
	return map[string]any{"kind": it.Kind, "id": it.ID, "seq": it.Seq, "rev": it.Rev, "deleted": it.Deleted, "blob": blob, "updated_at": it.UpdatedAt}
}

func (g *fakeGateway) serve(w http.ResponseWriter, r *http.Request) {
	g.mu.Lock()
	defer g.mu.Unlock()
	body, _ := io.ReadAll(r.Body)
	g.bodies = append(g.bodies, string(body))
	g.paths = append(g.paths, r.Method+" "+r.URL.RequestURI())
	if r.Header.Get("Authorization") != "Bearer "+g.token {
		g.fail(w, 401, "unauthorized")
		return
	}
	switch {
	case r.URL.Path == "/v1/sync/meta" && r.Method == "GET":
		g.send(w, g.metaBody())
	case r.URL.Path == "/v1/sync/meta" && r.Method == "PUT":
		var b struct {
			Salt, KDF, Check string
			Reset            bool
		}
		if json.Unmarshal(body, &b) != nil || len(b.Salt) < 22 || !fB64RE.MatchString(b.Salt) || !fB64RE.MatchString(b.Check) {
			g.fail(w, 400, "bad_value")
			return
		}
		if g.meta != nil && !b.Reset {
			if g.meta.Salt == b.Salt && g.meta.Check == b.Check && g.meta.KDF == b.KDF {
				g.send(w, g.metaBody())
				return
			}
			g.fail(w, 409, "already_set_up")
			return
		}
		if g.meta != nil {
			g.items, g.bytes = map[string]*fakeItem{}, 0
		}
		g.meta = &fakeMeta{Salt: b.Salt, KDF: b.KDF, Check: b.Check, Epoch: "ep" + strconv.FormatInt(g.seq+1, 10) + "x" + strconv.Itoa(len(g.bodies))}
		g.send(w, g.metaBody())
	case r.URL.Path == "/v1/sync/push" && r.Method == "POST":
		g.push(w, body)
	case r.URL.Path == "/v1/sync/pull" && r.Method == "GET":
		g.pull(w, r)
	case r.URL.Path == "/v1/sync" && r.Method == "DELETE":
		g.meta, g.items, g.bytes = nil, map[string]*fakeItem{}, 0
		g.send(w, map[string]any{"ok": true})
	default:
		g.fail(w, 405, "method_not_allowed")
	}
}

func (g *fakeGateway) push(w http.ResponseWriter, body []byte) {
	if g.meta == nil {
		g.fail(w, 409, "sync_not_set_up")
		return
	}
	if g.beforePush != nil {
		g.beforePush()
	}
	if g.busyOnce {
		g.busyOnce = false
		g.fail(w, 409, "busy")
		return
	}
	var b struct {
		Items []map[string]any `json:"items"`
	}
	if json.Unmarshal(body, &b) != nil {
		g.fail(w, 400, "bad_value")
		return
	}
	if len(b.Items) > 200 {
		g.fail(w, 413, "too_many_items")
		return
	}
	accepted, conflicts, rejected := []any{}, []any{}, []any{}
	seen := map[string]bool{}
	for _, o := range b.Items {
		kind, _ := o["kind"].(string)
		id, _ := o["id"].(string)
		echo := map[string]any{"kind": kind, "id": id}
		if !fKindRE.MatchString(kind) || !IDRe.MatchString(id) {
			echo["error"] = "bad_item"
			rejected = append(rejected, echo)
			continue
		}
		k := kind + "\x00" + id
		if seen[k] {
			echo["error"] = "duplicate"
			rejected = append(rejected, echo)
			continue
		}
		seen[k] = true
		br, ok := o["base_rev"].(float64)
		if !ok || br < 0 {
			echo["error"] = "bad_item"
			rejected = append(rejected, echo)
			continue
		}
		del, _ := o["deleted"].(bool)
		blob, _ := o["blob"].(string)
		if !del && (len(blob) < 40 || !fB64RE.MatchString(blob)) {
			echo["error"] = "bad_item"
			rejected = append(rejected, echo)
			continue
		}
		if !del && len(blob) > 512<<10 {
			echo["error"] = "too_large"
			rejected = append(rejected, echo)
			continue
		}
		row := g.items[k]
		var rev int64
		if row != nil {
			rev = row.Rev
		}
		if rev != int64(br) {
			if row != nil {
				conflicts = append(conflicts, wire(row))
			} else {
				conflicts = append(conflicts, map[string]any{"kind": kind, "id": id, "seq": 0, "rev": 0, "deleted": true, "blob": nil, "updated_at": 0})
			}
			continue
		}
		if del && row == nil {
			accepted = append(accepted, map[string]any{"kind": kind, "id": id, "rev": 0, "seq": 0})
			continue
		}
		old := int64(0)
		if row != nil && !row.Deleted {
			old = int64(len(row.Blob))
		}
		size := int64(len(blob))
		if del {
			size, blob = 0, ""
		}
		g.bytes += size - old
		g.seq++
		g.items[k] = &fakeItem{Kind: kind, ID: id, Seq: g.seq, Rev: rev + 1, Deleted: del, Blob: blob, UpdatedAt: 1_800_000_000_000 + g.seq}
		accepted = append(accepted, map[string]any{"kind": kind, "id": id, "rev": rev + 1, "seq": g.seq})
	}
	g.send(w, map[string]any{"accepted": accepted, "conflicts": conflicts, "rejected": rejected, "seq": g.seq, "bytes": g.bytes, "epoch": g.meta.Epoch})
}

func (g *fakeGateway) pull(w http.ResponseWriter, r *http.Request) {
	since, _ := strconv.ParseInt(r.URL.Query().Get("since"), 10, 64)
	lim, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	max := 200
	if g.pullLimit > 0 {
		max = g.pullLimit
	}
	if lim <= 0 || lim > max {
		lim = max
	}
	if g.meta == nil {
		g.send(w, map[string]any{"set_up": false, "items": []any{}, "next": since, "more": false})
		return
	}
	var rows []*fakeItem
	for _, it := range g.items {
		if it.Seq > since {
			rows = append(rows, it)
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Seq < rows[j].Seq })
	more := len(rows) > lim
	if more {
		rows = rows[:lim]
	}
	items := []any{}
	next := since
	for _, it := range rows {
		items = append(items, wire(it))
		next = it.Seq
	}
	g.send(w, map[string]any{"set_up": true, "items": items, "next": next, "more": more, "epoch": g.meta.Epoch, "seq": g.seq})
}

func (g *fakeGateway) allBodies() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return strings.Join(g.bodies, "\n")
}

func (g *fakeGateway) live(kind string) int {
	g.mu.Lock()
	defer g.mu.Unlock()
	n := 0
	for _, it := range g.items {
		if it.Kind == kind && !it.Deleted {
			n++
		}
	}
	return n
}
