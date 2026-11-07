package main

// Argus updates itself from the chat: the /update command and the update_argus tool
// (so "update yourself" in plain words works too). The frontend (selfupdate.js) shows
// the check, the download and the install as one live line, then Argus restarts.
// Background agents that were running pick up again after the restart.

import (
	"fmt"
	"github.com/lacksonkapapahue/argus-desktop/internal/brand"
	"os"
	"path/filepath"
	"time"

	"github.com/lacksonkapapahue/argus-desktop/internal/agents"
	"github.com/lacksonkapapahue/argus-desktop/internal/config"
	"github.com/lacksonkapapahue/argus-desktop/internal/llm"
)

// resumeMarker is written just before an update restarts Argus while agents run.
func resumeMarker() string { return filepath.Join(config.Dir(), "desktop", "resume-after-update") }

// updateArgusTool is the main agent's tool for updating the app itself.
func updateArgusTool() llm.Tool {
	var t llm.Tool
	t.Type = "function"
	t.Function.Name = "update_argus"
	t.Function.Description = "Update " + brand.NAME + " (this app) to the newest release. Use when the user asks you to update " + brand.NAME + " or yourself. It checks the " + brand.NAME + " downloads site, and if there's a new version " + brand.NAME + " downloads and installs it in the chat and restarts by itself. Never use a shell command to update " + brand.NAME + "."
	t.Function.Parameters = map[string]any{"type": "object", "properties": map[string]any{}}
	return t
}

// selfUpdate runs update_argus: check, then hand the install to the chat's update line.
func (a *App) selfUpdate() string {
	i := a.CheckUpdate()
	switch {
	case i.Error != "":
		return "Couldn't check for an " + brand.NAME + " update: " + i.Error + " Tell the user in one line."
	case !i.Available:
		return brand.NAME + " is already on the newest version, " + i.Current + ". Tell the user in one line."
	case !i.CanAuto:
		return fmt.Sprintf(brand.NAME+" %s is out (the user has %s), but there's no automatic install for this computer. Give them the release page: %s", i.Latest, i.Current, i.Page)
	}
	a.emit("selfupdate", map[string]any{"latest": i.Latest})
	return fmt.Sprintf("Updating to %s has started. "+brand.NAME+" shows the progress in the chat and restarts by itself once it's installed. Tell the user in one short line and call no more tools.", i.Latest)
}

// markResume remembers that agents were running when an update restarted Argus.
func (a *App) markResume() {
	if a.agents == nil {
		return
	}
	if r, q := a.agents.Running(); r+q > 0 {
		_ = os.MkdirAll(filepath.Dir(resumeMarker()), 0o755)
		_ = os.WriteFile(resumeMarker(), []byte(time.Now().Format(time.RFC3339)), 0o644)
	}
}

// resumeAfterUpdate restarts the agents an update interrupted (within the last hour).
func (a *App) resumeAfterUpdate() int {
	st, err := os.Stat(resumeMarker())
	if err != nil {
		return 0
	}
	_ = os.Remove(resumeMarker())
	if time.Since(st.ModTime()) > time.Hour {
		return 0
	}
	n := 0
	for _, v := range a.agents.List("") {
		if v.Status == agents.Interrupted && a.agents.Resume(v.Handle) == nil {
			n++
		}
	}
	return n
}
