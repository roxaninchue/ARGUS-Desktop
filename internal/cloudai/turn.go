package cloudai

// v0.19.1 (Blaze Mobile 0.45.1 TurnModel.kt): one route for the whole turn. Each step of a turn
// is its own gateway request, so after a rate limit or a health mark step 2 could land on another
// model than step 1, with other tool habits and a cold prompt cache. Every request of a turn now
// carries:
//
//   - X-Blaze-Turn: <id>, the same id for every step of the turn;
//   - X-Blaze-Prefer: provider/model, the route that answered the turn's first step, tried first
//     while it is healthy (Blaze Gateway 0.6.1; the gateway still falls back on an error).
//
// Both are plain headers: a gateway that doesn't read them ignores them.

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"sync"
)

const (
	TurnHeader   = "X-Blaze-Turn"
	PreferHeader = "X-Blaze-Prefer"
)

// Turn is one user turn's route memory. Safe for concurrent use.
type Turn struct {
	ID string

	mu       sync.Mutex
	pin      string
	last     string
	switches int
}

// NewTurn makes a turn with a random id.
func NewTurn() *Turn {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return &Turn{ID: "t" + hex.EncodeToString(b)}
}

// Served records the route ("provider/model") that answered one step; "" is ignored.
func (t *Turn) Served(route string) {
	r := strings.TrimSpace(route)
	if t == nil || r == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.pin == "" {
		t.pin = r
	}
	if t.last != "" && t.last != r {
		t.switches++
	}
	t.last = r
}

// Pin is the route of the turn's first answered step ("" until then).
func (t *Turn) Pin() string {
	if t == nil {
		return ""
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.pin
}

// Switches counts steps answered by another route than the step before.
func (t *Turn) Switches() int {
	if t == nil {
		return 0
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.switches
}

// Apply sets the turn headers on a gateway request.
func (t *Turn) Apply(h http.Header) {
	if t == nil {
		return
	}
	if id := headerSafe(t.ID); id != "" {
		h.Set(TurnHeader, id)
	}
	if p := headerSafe(t.Pin()); p != "" {
		h.Set(PreferHeader, p)
	}
}

// headerSafe keeps visible ASCII only, at most 120 characters.
func headerSafe(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '!' && r <= '~' {
			b.WriteRune(r)
			if b.Len() >= 120 {
				break
			}
		}
	}
	return b.String()
}

type turnKey struct{}

// WithTurn attaches t to ctx; every gateway request made with that ctx carries the turn headers.
func WithTurn(ctx context.Context, t *Turn) context.Context {
	return context.WithValue(ctx, turnKey{}, t)
}

// TurnFrom is the turn attached to ctx, or nil.
func TurnFrom(ctx context.Context) *Turn {
	t, _ := ctx.Value(turnKey{}).(*Turn)
	return t
}
