package main

import (
	"os/exec"
	"syscall"

	"github.com/lacksonkapapahue/argus-desktop/internal/studio"
)

func init() {
	studio.HideBrowserWindow = func(c *exec.Cmd) {
		c.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000} // CREATE_NO_WINDOW
	}
}
