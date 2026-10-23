package llm

import (
	"strings"
	"testing"
)

// Keep-alive comments and empty heartbeat chunks never count as progress for the stall
// watchdog; output (text, finish) does.
func TestStreamTickOnlyOnData(t *testing.T) {
	in := ": ping\n\ndata: {\"choices\":[{\"delta\":{}}]}\n\n: ping\n\ndata: {\"choices\":[{\"delta\":{\"role\":\"assistant\"}}]}\n\n" +
		"data: {\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}\n\n: ping\n\ndata: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n"
	n := 0
	res, err := ParseStream(strings.NewReader(in), func(string) {}, func() { n++ })
	if err != nil || res == nil || !strings.Contains(res.Text, "hi") {
		t.Fatalf("parse: %v %+v", err, res)
	}
	if n != 2 {
		t.Fatalf("ticks: %d, want 2 (text and finish only)", n)
	}
}
