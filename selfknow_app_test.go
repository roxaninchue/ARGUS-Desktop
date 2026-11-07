package main

import (
	"os"
	"strings"
	"testing"

	"github.com/lacksonkapapahue/argus-desktop/internal/config"
	"github.com/lacksonkapapahue/argus-desktop/internal/selfknow"
	"github.com/lacksonkapapahue/argus-desktop/internal/sysinfo"
	"github.com/lacksonkapapahue/argus-desktop/internal/tools"
)

func selfTestApp(t *testing.T) *App {
	// Not t.TempDir: store.Default keeps argus.db open, and Windows can't delete an open file.
	home, err := os.MkdirTemp("", "argus-selfknow-")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("ARGUS_HOME", home)
	dir := t.TempDir()
	return &App{cfg: &config.Config{}, pv: &preview{}, info: sysinfo.Info{OS: "linux", OSName: "Arch Linux", Arch: "amd64", Cores: 8, RAMGB: 16},
		env: &tools.Env{Root: dir, Shell: tools.NewShell("sh", "/bin/sh", dir)}}
}

// Luke asked "how do I connect my phone?" and Argus asked which apps he was building.
// The system prompt must say who Argus is and carry the rule against that.
func TestSystemPromptHasAboutYou(t *testing.T) {
	a := selfTestApp(t)
	sys := a.system("how do I connect my phone to the desktop app?")
	for _, want := range []string{"ABOUT YOU", "You are Argus, running here as Argus Desktop", "Argus Android", "Argus CLI", "Phone ↔ PC (Argus Link)",
		"ArgusRouter:", "Arch Linux", selfknow.ToolName, "Never ask the user which apps they are building"} {
		if !strings.Contains(sys, want) {
			t.Errorf("system prompt misses %q", want)
		}
	}
	if !tools.IsCore(selfknow.ToolName) {
		t.Error("about_argus should be a core tool")
	}
}

func TestAboutArgusTool(t *testing.T) {
	a := selfTestApp(t)
	old := theApp
	theApp = a
	defer func() { theApp = old }()
	full := tools.Run(a.env, selfknow.ToolName, `{}`)
	for _, want := range []string{"ARGUS SELF-REPORT", "IDENTITY", "ARGUS LINK", "Settings › Argus Desktop", "ARGUSROUTER", "auto/best-free", "SLASH COMMANDS", "/views", "VIEWS", "ArgusRouter", "DATA FOLDERS", "Arch Linux"} {
		if !strings.Contains(full, want) {
			t.Errorf("about_argus misses %q:\n%s", want, full)
		}
	}
	link := tools.Run(a.env, selfknow.ToolName, `{"section":"link"}`)
	if !strings.HasPrefix(link, "ARGUS LINK") || strings.Contains(link, "SLASH COMMANDS") {
		t.Errorf("section link: %s", link)
	}
}
