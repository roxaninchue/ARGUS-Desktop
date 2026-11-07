package main

import (
	_ "embed"
	"github.com/lacksonkapapahue/argus-desktop/internal/brand"
	"sync"

	"fyne.io/systray"
)

// Tray icons: the pixel "B" drawn at integer cell sizes (build/tray, made from logo.svg).
var (
	//go:embed build/tray/tray.ico
	trayICO []byte
	//go:embed build/tray/tray.png
	trayPNG []byte
	//go:embed build/tray/tray-template.png
	trayTemplate []byte
)

var (
	trayMu     sync.Mutex
	trayStatus *systray.MenuItem
	trayRouter *systray.MenuItem
)

// trayReady builds the menu: Quick ask, Open Argus, a status line, Quit Argus.
func (a *App) trayReady() {
	setTrayIcon()
	systray.SetTooltip(brand.NAME)
	ask := systray.AddMenuItem("Quick ask", "Open the "+brand.NAME+" orb")
	open := systray.AddMenuItem("Open "+brand.NAME, "Show the "+brand.NAME+" window")
	br := systray.AddMenuItem(brand.NAME+"Router dashboard", "Open "+brand.NAME+"Router's dashboard in your browser")
	systray.AddSeparator()
	st := systray.AddMenuItem("Idle", "What "+brand.NAME+" is doing")
	st.Disable()
	systray.AddSeparator()
	quit := systray.AddMenuItem("Quit "+brand.NAME, "Stop "+brand.NAME+" completely")
	trayMu.Lock()
	trayStatus = st
	trayRouter = br
	br.Hide() // shown once ArgusRouter is on (setTrayRouter)
	trayMu.Unlock()
	a.sh.mu.Lock()
	a.sh.trayOK = trayAvailable()
	if !a.sh.trayOK {
		a.sh.trayErr = "No system tray found (on GNOME, install the AppIndicator extension)"
	}
	a.sh.mu.Unlock()
	go func() {
		for {
			select {
			case <-ask.ClickedCh:
				a.ShowOrb()
			case <-open.ClickedCh:
				a.ShowMain()
			case <-br.ClickedCh:
				a.OpenRouterDashboard()
			case <-quit.ClickedCh:
				a.quit()
				return
			}
		}
	}()
}

// setTrayStatus updates the disabled status line in the tray menu.
func setTrayStatus(s string) {
	trayMu.Lock()
	st := trayStatus
	trayMu.Unlock()
	if st != nil {
		st.SetTitle(s)
		systray.SetTooltip(brand.NAME + " · " + s)
	}
}

// setTrayRouter shows ArgusRouter's state on its tray item.
func setTrayRouter(v RouterView) {
	trayMu.Lock()
	it := trayRouter
	trayMu.Unlock()
	if it == nil {
		return
	}
	if v.Enabled {
		it.Show()
	} else {
		it.Hide() // Argus Cloud answers; the local router is an advanced option
	}
	dot := map[string]string{"green": "●", "amber": "◐", "red": "○"}[v.Level]
	it.SetTitle(dot + " " + brand.NAME + "Router dashboard")
	it.SetTooltip(v.Note)
}
