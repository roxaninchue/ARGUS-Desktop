package llm

import (
	"context"
	"strings"
	"testing"
)

var toolSet = []string{"about_argus", "web_search", "fetch_page", "run_shell"}

// Calls a model wrote as text (non-gateway routes) become real calls (v0.19.13, ported from CLI 0.14.14).
func TestTextCallsUserCases(t *testing.T) {
	for _, in := range []string{
		"dispatch_agent: about_argus {\"topic\":\"crew\"}",
		`{"tool":"about_argus","args":{"topic":"crew"}}`,
		"```json\n{\"name\":\"about_argus\",\"arguments\":{\"topic\":\"crew\"}}\n```",
		"<tool_call>{\"name\":\"about_argus\",\"arguments\":{\"topic\":\"crew\"}}</tool_call>",
	} {
		rest, calls := ExtractTextCalls(in, toolSet)
		if len(calls) != 1 || calls[0].Function.Name != "about_argus" || !strings.Contains(calls[0].Function.Arguments, `"crew"`) {
			t.Errorf("%q: calls %+v", in, calls)
			continue
		}
		if strings.TrimSpace(rest) != "" {
			t.Errorf("%q: raw syntax left in the answer: %q", in, rest)
		}
	}
	if _, calls := ExtractTextCalls("Argus has a crew of agents. Ask me about it.", toolSet); len(calls) != 0 {
		t.Fatalf("prose became a call: %+v", calls)
	}
}

// The review's D2 case: run_shell written as a <tool_call> block by an own-key Groq model.
func TestTextCallRunShell(t *testing.T) {
	in := "I'll run the tests.\n<tool_call>{\"name\": \"run_shell\", \"arguments\": {\"command\": \"go test ./...\"}}</tool_call>"
	rest, calls := ExtractTextCalls(in, toolSet)
	if len(calls) != 1 || calls[0].Function.Name != "run_shell" || !strings.Contains(calls[0].Function.Arguments, "go test") {
		t.Fatalf("calls %+v", calls)
	}
	if strings.Contains(rest, "tool_call") {
		t.Fatalf("syntax left: %q", rest)
	}
}

func TestTextGuardHidesSyntax(t *testing.T) {
	var shown strings.Builder
	g := &TextGuard{Known: toolSet, Out: func(s string) { shown.WriteString(s) }}
	for _, c := range []string{"Let me check.\n", "<tool_call>", `{"name":"web_search","arguments":{"query":"x"}}`, "</tool_call>"} {
		g.Write(c)
	}
	held := g.Held()
	if strings.Contains(shown.String(), "web_search") || strings.Contains(shown.String(), "<tool_call") {
		t.Fatalf("syntax shown: %q", shown.String())
	}
	if !strings.HasPrefix(shown.String(), "Let me check.") || !strings.Contains(held, "web_search") {
		t.Fatalf("shown %q held %q", shown.String(), held)
	}
}

func TestRequireToolContext(t *testing.T) {
	if RequireTool(context.Background()) {
		t.Fatal("plain ctx requires a tool")
	}
	if !RequireTool(WithRequireTool(context.Background())) {
		t.Fatal("flag lost")
	}
	if ToolChoice(context.Background()) != "auto" || ToolChoice(WithRequireTool(context.Background())) != "required" {
		t.Fatal("tool_choice")
	}
}

func TestInlineEnvelopeWithStringInput(t *testing.T) {
	old := PrimaryArg
	PrimaryArg = func(n string) string {
		if n == "memory_search" {
			return "query"
		}
		return ""
	}
	defer func() { PrimaryArg = old }()
	rest, calls := ExtractTextCalls(`Let's call memory_search.{"tool":"memory_search","input":"sister"}`, []string{"memory_search"})
	if len(calls) != 1 || calls[0].Function.Name != "memory_search" || calls[0].Function.Arguments != `{"query":"sister"}` {
		t.Fatalf("calls %+v", calls)
	}
	if strings.Contains(rest, "{") {
		t.Fatalf("syntax left: %q", rest)
	}
	// Plain prose holding JSON data is not a call.
	if _, c := ExtractTextCalls(`The config is {"name":"argus","port":8080}.`, []string{"memory_search"}); len(c) != 0 {
		t.Fatalf("data read as call: %+v", c)
	}
}
