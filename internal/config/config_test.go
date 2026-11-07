package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/lacksonkapapahue/argus-desktop/internal/keystore"
)

type noKeychain struct{}

func (noKeychain) Get(string) (string, error) { return "", errors.New("none") }
func (noKeychain) Set(string, string) error   { return errors.New("none") }
func (noKeychain) Delete(string) error        { return nil }

func TestSaveKeepsDesktopFields(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ARGUS_HOME", dir)
	path := filepath.Join(dir, "config.json")
	in := `{"chain":[{"provider":"groq","model":"m"}],"persona":"old","model_for":{"groq":"x"},"default":{"provider":"auto","model":""},"keys":{"groq":"legacy"}}`
	if err := os.WriteFile(path, []byte(in), 0o600); err != nil {
		t.Fatal(err)
	}
	c := Load()
	c.Persona = "" // cleared omitempty field must disappear
	c.MaxSteps = 12
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	b, _ := os.ReadFile(path)
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if out["model_for"] == nil || out["default"] == nil || out["keys"] == nil {
		t.Fatalf("desktop/legacy fields dropped: %s", b)
	}
	if _, ok := out["persona"]; ok || out["max_steps"].(float64) != 12 {
		t.Fatalf("own fields wrong: %s", b)
	}
}

func TestKeysGoToSharedStore(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ARGUS_HOME", dir)
	t.Setenv("GROQ_API_KEY", "")
	old := keystore.Keychain
	keystore.Keychain = noKeychain{}
	defer func() { keystore.Keychain = old }()
	c := Load()
	where, err := c.SetKey("groq", "gsk_1")
	if err != nil || where != "~/.argus/keys.json (owner-only)" {
		t.Fatalf("%q %v", where, err)
	}
	if Load().Key("groq") != "gsk_1" {
		t.Fatal("key not read back")
	}
	if _, err := os.Stat(filepath.Join(dir, "config.json")); err == nil {
		b, _ := os.ReadFile(filepath.Join(dir, "config.json"))
		var m map[string]any
		_ = json.Unmarshal(b, &m)
		if m["keys"] != nil {
			t.Fatalf("key written to config.json: %s", b)
		}
	}
	c.DeleteKey("groq")
	if Load().Key("groq") != "" {
		t.Fatal("key not deleted")
	}
}
