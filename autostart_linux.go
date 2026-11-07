package main

import (
	"fmt"
	"github.com/lacksonkapapahue/argus-desktop/internal/brand"
	"os"
	"path/filepath"
	"strings"
)

func autostartFile() string {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "autostart", "argus-desktop.desktop")
}

// setAutostart writes ~/.config/autostart/argus-desktop.desktop.
func setAutostart(on bool) error {
	p := autostartFile()
	if !on {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	exe := selfPath()
	if strings.ContainsAny(exe, " \"'\\") {
		exe = `"` + strings.ReplaceAll(exe, `"`, `\"`) + `"`
	}
	entry := fmt.Sprintf("[Desktop Entry]\nType=Application\nName="+brand.NAME+"\nComment="+brand.NAME+" assistant, in the background\nExec=%s --background\nIcon=argus\nTerminal=false\nX-GNOME-Autostart-enabled=true\n", exe)
	return os.WriteFile(p, []byte(entry), 0o644)
}

func autostartEnabled() bool {
	b, err := os.ReadFile(autostartFile())
	return err == nil && strings.Contains(string(b), selfPath())
}
