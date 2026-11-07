package connect

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func readJSONFile(t *testing.T, p string) map[string]any {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	var v map[string]any
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatalf("%v\n%s", err, b)
	}
	return v
}

func servers(t *testing.T, p string) map[string]any {
	t.Helper()
	s, _ := readJSONFile(t, p)["mcpServers"].(map[string]any)
	return s
}

// The standard shape Argus CLI writes is read as-is.
func TestMCPConfigRead(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "mcp.json")
	os.WriteFile(cfg, []byte(`{
  "mcpServers": {
    "files": {"command": "npx", "args": ["-y", "@modelcontextprotocol/server-filesystem", "~"], "env": {"DEBUG": "1"}},
    "linear": {"url": "https://mcp.linear.app/mcp", "headers": {"Authorization": "Bearer ${LINEAR_TOKEN}"}},
    "Old_SSE": {"url": "https://example.com/sse"},
    "typed": {"type": "sse", "url": "https://example.com/events", "disabled": true},
    "cli-only": {"command": "x", "cliExtra": {"keep": true}}
  },
  "otherTool": {"keep": "me"}
}`), 0o600)
	m := mcpManager(t, &recorder{})
	if err := m.SetMCPConfig(cfg); err != nil {
		t.Fatal(err)
	}
	by := map[string]MCPServer{}
	for _, s := range m.Store().MCPList() {
		by[s.ID] = s
	}
	if len(by) != 5 {
		t.Fatalf("%d servers: %+v", len(by), by)
	}
	f := by["files"]
	if f.Transport != "stdio" || f.Command != "npx" || len(f.Args) != 3 || f.Env["DEBUG"] != "1" {
		t.Fatalf("files: %+v", f)
	}
	if l := by["linear"]; l.Transport != "http" || l.Headers["Authorization"] != "Bearer ${LINEAR_TOKEN}" {
		t.Fatalf("linear: %+v", l)
	}
	if o := by["old-sse"]; o.Transport != "sse" || o.Key != "Old_SSE" {
		t.Fatalf("Old_SSE: %+v", o)
	}
	if ty := by["typed"]; ty.Transport != "sse" || !ty.Disabled {
		t.Fatalf("typed: %+v", ty)
	}
	v := m.Views()
	if v.MCPFile != cfg || v.MCPError != "" {
		t.Fatalf("view file %q err %q", v.MCPFile, v.MCPError)
	}
	for _, sv := range v.MCP {
		if sv.ID == "linear" && (len(sv.HeaderKeys) != 1 || sv.HeaderKeys[0] != "Authorization") {
			t.Fatalf("header keys: %+v", sv)
		}
	}
}

// Desktop's changes land in the file in the same shape, keeping what it doesn't know.
func TestMCPConfigWriteThrough(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "mcp.json")
	os.WriteFile(cfg, []byte(`{"mcpServers": {"cli-only": {"command": "x", "cliExtra": {"keep": true}}}, "otherTool": 1}`), 0o600)
	f, rec := newFakeMCP(t), &recorder{}
	m := mcpManager(t, rec)
	if err := m.SetMCPConfig(cfg); err != nil {
		t.Fatal(err)
	}
	v, err := m.AddMCP(MCPSpec{Name: "Fake Tools", Transport: "http", URL: f.srv.URL + "/mcp", Token: "sekret", Headers: "X-Team: argus\nbad line"})
	if err != nil || v.Status != "ready" {
		t.Fatalf("%v %+v", err, v)
	}
	if v.ID != "fake-tools" {
		t.Fatalf("id %q", v.ID)
	}
	raw, _ := os.ReadFile(cfg)
	if strings.Contains(string(raw), "sekret") {
		t.Fatal("the keychain token leaked into mcp.json")
	}
	top := readJSONFile(t, cfg)
	if top["otherTool"] == nil {
		t.Fatal("unknown top-level key dropped")
	}
	s := servers(t, cfg)
	ft, _ := s["fake-tools"].(map[string]any)
	if ft["url"] != f.srv.URL+"/mcp" || ft["headers"].(map[string]any)["X-Team"] != "argus" || ft["type"] != nil || ft["command"] != nil {
		t.Fatalf("written entry: %v", ft)
	}
	if co, _ := s["cli-only"].(map[string]any); co["cliExtra"] == nil {
		t.Fatal("another app's server lost its extra fields")
	}
	// Off for Argus → "disabled": true; on again removes it.
	_ = m.SetMCPEnabled(v.ID, false)
	if servers(t, cfg)["fake-tools"].(map[string]any)["disabled"] != true {
		t.Fatal("disabled not written")
	}
	_ = m.SetMCPEnabled(v.ID, true)
	if _, ok := servers(t, cfg)["fake-tools"].(map[string]any)["disabled"]; ok {
		t.Fatal("disabled not cleared")
	}
	// Tool modes and the tools cache stay out of the shared file.
	_ = m.SetToolMode("mcp:"+v.ID, "echo", "always")
	if strings.Contains(string(mustRead(t, cfg)), "always") || strings.Contains(string(mustRead(t, cfg)), "wipe_all") {
		t.Fatal("desktop-only state leaked into mcp.json")
	}
	// Remove: gone from the file, the other server kept.
	if err := m.RemoveMCP(v.ID); err != nil {
		t.Fatal(err)
	}
	s = servers(t, cfg)
	if _, ok := s["fake-tools"]; ok || s["cli-only"] == nil {
		t.Fatalf("after remove: %v", s)
	}
}

func mustRead(t *testing.T, p string) []byte {
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// Another app (Argus CLI) edits the file: Desktop follows on its next look.
func TestMCPConfigFollowsOtherApp(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "mcp.json")
	os.WriteFile(cfg, []byte(`{"mcpServers": {"a": {"command": "x"}, "b": {"url": "https://b.example/mcp"}}}`), 0o600)
	m := mcpManager(t, &recorder{})
	_ = m.SetMCPConfig(cfg)
	if n := len(m.Store().MCPList()); n != 2 {
		t.Fatalf("%d", n)
	}
	time.Sleep(20 * time.Millisecond)
	os.WriteFile(cfg, []byte(`{"mcpServers": {"b": {"url": "https://b.example/v2/mcp"}, "c": {"command": "y", "args": ["--z"]}}}`), 0o600)
	got := map[string]MCPServer{}
	for _, s := range m.Store().MCPList() {
		got[s.ID] = s
	}
	if _, ok := got["a"]; ok || got["b"].URL != "https://b.example/v2/mcp" || got["c"].Command != "y" || len(got) != 2 {
		t.Fatalf("%+v", got)
	}
	// A broken file is reported, and the last good list stays.
	time.Sleep(20 * time.Millisecond)
	os.WriteFile(cfg, []byte(`{"mcpServers": {`), 0o600)
	if n := len(m.Store().MCPList()); n != 2 {
		t.Fatalf("broken file dropped servers: %d", n)
	}
	if !strings.Contains(m.Views().MCPError, "mcp.json") {
		t.Fatal("broken file not reported")
	}
}

// Servers added before v0.8.1 move into the shared file once.
func TestMCPConfigMigrates(t *testing.T) {
	dir := t.TempDir()
	m := mcpManager(t, &recorder{})
	if err := m.Store().PutMCP(MCPServer{ID: "legacy", Name: "legacy", Transport: "stdio", Command: "npx", Args: []string{"-y", "srv"}}); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(dir, "mcp.json")
	if err := m.SetMCPConfig(cfg); err != nil {
		t.Fatal(err)
	}
	l, _ := servers(t, cfg)["legacy"].(map[string]any)
	if l["command"] != "npx" || len(l["args"].([]any)) != 2 {
		t.Fatalf("not migrated: %v", l)
	}
	if n := len(m.Store().MCPList()); n != 1 {
		t.Fatal(n)
	}
}

// env from the file reaches a local server (the fake server only answers when it
// sees ARGUS_FAKE_MCP=1), and ${VAR} expands.
func TestMCPConfigStdioEnv(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Skip(err)
	}
	t.Setenv("ARGUS_TEST_ONE", "1")
	dir := t.TempDir()
	cfg := filepath.Join(dir, "mcp.json")
	b, _ := json.Marshal(map[string]any{"mcpServers": map[string]any{"local": map[string]any{"command": exe, "args": []string{"-test.run=none"}, "env": map[string]string{"ARGUS_FAKE_MCP": "${ARGUS_TEST_ONE}"}}}})
	os.WriteFile(cfg, b, 0o600)
	m := mcpManager(t, &recorder{})
	_ = m.SetMCPConfig(cfg)
	v, err := m.RefreshMCP(context.Background(), "local")
	if err != nil || v.Status != "ready" || len(v.Tools) != 2 {
		t.Fatalf("%v %+v", err, v)
	}
	if v.EnvKeys[0] != "ARGUS_FAKE_MCP" {
		t.Fatal(v.EnvKeys)
	}
}

func TestMCPIDAndEntry(t *testing.T) {
	for in, want := range map[string]string{"GitHub": "github", "my_server": "my-server", "  a  b ": "a-b", "": "server", "ünï": "n", strings.Repeat("x", 40): strings.Repeat("x", 24)} {
		if got := MCPID(in); got != want {
			t.Errorf("MCPID(%q) = %q, want %q", in, got, want)
		}
	}
	if (MCPEntry{URL: "https://x/mcp"}).Transport() != "http" || (MCPEntry{URL: "https://x/sse"}).Transport() != "sse" ||
		(MCPEntry{URL: "https://x/sse", Type: "http"}).Transport() != "http" || (MCPEntry{Command: "x"}).Transport() != "stdio" ||
		(MCPEntry{URL: "https://x", Type: "streamable-http"}).Transport() != "http" {
		t.Fatal("transport")
	}
	if n := mcpToolName("github", "create_issue"); n != "mcp__github__create_issue" {
		t.Fatal(n)
	}
}
