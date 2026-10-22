package main

import (
	"encoding/json"
	"testing"

	"github.com/lacksonkapapahue/blaze-desktop/internal/agents"
	"github.com/lacksonkapapahue/blaze-desktop/internal/llm"
	"github.com/lacksonkapapahue/blaze-desktop/internal/tools"
)

// v0.19.13: every tool Blaze offers must have a schema strict validators accept. list_agents sent
// "required": null, so Blaze Cloud upstreams answered 400 ("invalid JSON schema for tool
// list_agents") and Desktop showed "busy" on most turns.
func TestEveryToolSchemaIsStrictValid(t *testing.T) {
	all := append(append(append(tools.All(), agents.MainTools()...), agents.SpecTools()...), updateBlazeTool(), weatherTool())
	for _, tl := range all {
		checkToolSchema(t, tl)
	}
}

func checkToolSchema(t *testing.T, tl llm.Tool) {
	t.Helper()
	b, err := json.Marshal(tl.Function.Parameters)
	if err != nil {
		t.Fatalf("%s: %v", tl.Function.Name, err)
	}
	var p map[string]any
	if err := json.Unmarshal(b, &p); err != nil || p["type"] != "object" {
		t.Fatalf("%s: parameters not an object schema: %s", tl.Function.Name, b)
	}
	if _, ok := p["properties"].(map[string]any); !ok {
		t.Fatalf("%s: properties must be an object: %s", tl.Function.Name, b)
	}
	if r, present := p["required"]; present {
		if _, ok := r.([]any); !ok {
			t.Fatalf("%s: required must be an array, got %s", tl.Function.Name, b)
		}
	}
}
