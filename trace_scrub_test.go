package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/lacksonkapapahue/argus-desktop/internal/cloudai"
)

// v0.19.13: the trace event never carries the upstream provider or model of an Argus Cloud attempt.
func TestTraceScrubsCloudUpstream(t *testing.T) {
	in := []cloudai.Step{{Provider: "groq", Model: "llama-3.3-70b-versatile", RequestID: "r1", Status: 200},
		{Provider: "openrouter", Model: "deepseek/deepseek-chat-v3:free", RequestID: "r2"}}
	out := scrubAttempts(in)
	b, _ := json.Marshal(out)
	for _, leak := range []string{"groq", "llama", "openrouter", "deepseek"} {
		if strings.Contains(strings.ToLower(string(b)), leak) {
			t.Fatalf("trace leaks %q: %s", leak, b)
		}
	}
	if out[0].Provider != cloudai.Name || out[0].Model != cloudai.Model || out[1].RequestID != "r2" {
		t.Fatalf("bad scrub: %+v", out)
	}
	if in[0].Provider != "groq" {
		t.Fatal("scrub must not mutate the turn's own record")
	}
}
