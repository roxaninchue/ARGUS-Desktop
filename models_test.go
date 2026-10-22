package main

import (
	"encoding/json"
	"github.com/lacksonkapapahue/blaze-desktop/internal/cloudai"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lacksonkapapahue/blaze-desktop/internal/config"
	"github.com/lacksonkapapahue/blaze-desktop/internal/llm"
)

func testApp(t *testing.T) *App {
	t.Helper()
	t.Setenv("BLAZE_HOME", t.TempDir())
	a := NewApp()
	a.cfg = config.Load()
	close(a.ready)
	return a
}

// A v0.2 config.json (Pollinations in the chain, no default) loads unchanged and routes safely.
func TestOldConfigStillLoads(t *testing.T) {
	home := t.TempDir()
	t.Setenv("BLAZE_HOME", home)
	old := `{"chain":[{"provider":"gemini","model":"gemini-3.8-flash"},{"provider":"pollinations","model":"openai-fast"}],"auto_approve":true,"setup_done":true,"max_steps":30,"persona":"short"}`
	if err := os.WriteFile(filepath.Join(home, "config.json"), []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	c := config.Load()
	if !c.IsAuto() || !c.AutoApprove || c.Persona != "short" || len(c.Chain) != 2 {
		t.Fatalf("old config not read: %+v", c)
	}
	for _, e := range c.RouteChain() {
		if e.Provider == config.KeylessID {
			t.Fatal("keyless must not be a chain entry")
		}
	}
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(home, "config.json"))
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	for _, k := range []string{"chain", "auto_approve", "setup_done", "max_steps", "persona"} {
		if _, ok := m[k]; !ok {
			t.Fatalf("field %s lost on save: %s", k, b)
		}
	}
	if _, ok := m["default"]; ok {
		t.Fatalf("new optional fields should be omitted when unset: %s", b)
	}
}

func TestSelectModelRules(t *testing.T) {
	a := testApp(t)
	t.Setenv("GROQ_API_KEY", "gsk_test")
	a.SelectModel(config.KeylessID, "openai-fast")
	if !a.cfg.IsAuto() {
		t.Fatal("Pollinations must never become the default")
	}
	a.SelectModel("gemini", "gemma-3-27b-it")
	if !a.cfg.IsAuto() {
		t.Fatal("a model that can't call tools must never become the default")
	}
	a.SelectModel("groq", "qwen/qwen3-32b")
	if a.cfg.IsAuto() || a.cfg.Default.Provider != "groq" || a.cfg.Default.Model != "qwen/qwen3-32b" {
		t.Fatalf("default not set: %+v", a.cfg.Default)
	}
	if len(a.cfg.Chain) != 1 || a.cfg.Chain[0].Model != "qwen/qwen3-32b" {
		t.Fatalf("chain not updated: %+v", a.cfg.Chain)
	}
	if got := a.modelName(); got != "Groq · qwen/qwen3-32b" {
		t.Fatalf("chip label %q", got)
	}
	ts := llm.Targets(a.cfg, true)
	if len(ts) == 0 || ts[0].Provider != "groq" || !ts[0].Fixed {
		t.Fatalf("fixed model not first: %+v", ts)
	}
	a.SelectModel("auto", "")
	if !a.cfg.IsAuto() {
		t.Fatal("auto not restored")
	}
}

func TestModelsMenuNeverOffersKeyless(t *testing.T) {
	a := testApp(t)
	t.Setenv("GROQ_API_KEY", "gsk_test")
	a.cfg.NoKeyless = false
	items := a.Models()
	// v0.19.13: the Blaze Cloud row (blaze-auto) leads, then BlazeRouter and the Auto chain (Custom mode).
	if len(items) < 4 || items[0].Provider != cloudai.ProviderID || items[0].Model != cloudai.Model || items[1].Provider != config.RouterID || items[2].Provider != config.Auto {
		t.Fatalf("Blaze Cloud, BlazeRouter, then auto must lead: %+v", items)
	}
	for _, it := range items {
		if it.Provider == config.KeylessID {
			t.Fatal("keyless offered in /model")
		}
	}
	if !strings.Contains(a.modelName(), "Groq") {
		t.Fatalf("chip should name the first tool-capable model, got %q", a.modelName())
	}
}

func TestAddProviderRefusesKeylessAndWrongKey(t *testing.T) {
	a := testApp(t)
	if r := a.AddProvider(config.KeylessID, ""); r.OK {
		t.Fatal("keyless must not be added as a model")
	}
	if r := a.AddProvider("groq", "AIzaSyD-not-a-groq-key-0000000000000000"); r.OK || !strings.Contains(r.Error, "Gemini") {
		t.Fatalf("wrong-provider key accepted: %+v", r)
	}
}

func TestTurnErrorIsPlain(t *testing.T) {
	e := turnError(&llm.RouteError{Failures: []llm.Failure{{Name: "Google Gemini", Code: 400, Message: llm.PlainText("Google Gemini", 400, "Request contains an invalid argument."), Raw: "HTTP 400: Request contains an invalid argument."}}})
	if msg := e["message"].(string); strings.Contains(msg, "HTTP 400") || !strings.Contains(msg, "Google Gemini") {
		t.Fatalf("message not plain: %q", msg)
	}
	if !strings.Contains(e["detail"].(string), "HTTP 400") {
		t.Fatal("raw detail should be kept for the Details toggle")
	}
	if e := turnError(&llm.RouteError{NoModels: true}); e["kind"] != "setup" {
		t.Fatal("no-model errors should offer setup")
	}
}

// v0.19.13: in Cloud mode the menu offers Blaze Cloud as blaze-auto only, plus the user's own keyed
// providers; no BlazeRouter/Auto rows and no keyless/unkeyed Blaze-side providers.
func TestModelsMenuCloudModeBlazeAutoOnly(t *testing.T) {
	a := testApp(t)
	t.Setenv("GROQ_API_KEY", "gsk_test")
	old := cloudModels
	t.Cleanup(func() { cloudModels = old })
	ch := cloudai.LoadChoice(t.TempDir(), false)
	ch.Mode = cloudai.ModeCloud
	cloudModels = cloudai.NewRouter(ch, cloudai.NewGateway(cloudai.BaseFrom(""), func() string { return "" }))
	items := a.Models()
	if len(items) == 0 || items[0].Provider != cloudai.ProviderID || items[0].Model != cloudai.Model || !items[0].Current {
		t.Fatalf("Blaze Cloud (blaze-auto, current) must lead: %+v", items)
	}
	for _, it := range items[1:] {
		if it.Provider == config.RouterID || it.Provider == config.Auto || it.Provider == config.KeylessID || it.Current {
			t.Fatalf("Cloud mode offers %+v", it)
		}
		if it.Provider != "groq" && it.Provider != "ollama" && it.Provider != config.Custom {
			t.Fatalf("Cloud mode offers a provider the user has no key for: %+v", it)
		}
	}
	a.SelectModel("groq", "")
	if cloudModelsOn() {
		t.Fatal("picking an own provider must switch to Custom")
	}
	a.SelectModel(cloudai.ProviderID, "")
	if !cloudModelsOn() {
		t.Fatal("picking the Blaze Cloud row must return to Cloud")
	}
}
