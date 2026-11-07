package main

import (
	"github.com/lacksonkapapahue/argus-desktop/internal/brand"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const runKey = `Software\Microsoft\Windows\CurrentVersion\Run`

// setAutostart writes HKCU\...\Run "Argus" = "<exe>" --background.
func setAutostart(on bool) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.SET_VALUE|registry.QUERY_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	if !on {
		if err := k.DeleteValue(brand.NAME); err != nil && err != registry.ErrNotExist {
			return err
		}
		return nil
	}
	return k.SetStringValue(brand.NAME, `"`+selfPath()+`" --background`)
}

func autostartEnabled() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	v, _, err := k.GetStringValue(brand.NAME)
	return err == nil && strings.Contains(strings.ToLower(v), strings.ToLower(selfPath()))
}
