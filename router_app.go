package main

// ArgusRouter in the app: the local OpenAI-compatible gateway (internal/router) starts
// with Argus, shows as a button with a status dot in the top bar and the tray, opens
// its dashboard inside Argus or in the browser, and is what the model picker offers
// first ("ArgusRouter · auto/best-free"). Keys and models are managed on its
// dashboard; they still live in Argus's shared key store, so Argus CLI keeps working.

import (
	"errors"
	"github.com/lacksonkapapahue/argus-desktop/internal/brand"
	"strings"
	"time"

	"github.com/lacksonkapapahue/argus-desktop/internal/config"
	"github.com/lacksonkapapahue/argus-desktop/internal/llm"
	"github.com/lacksonkapapahue/argus-desktop/internal/router"
	wr "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) startRouter() {
	a.router = router.New(router.Options{
		Config:  func() *config.Config { return a.cfg },
		Version: version,
		Changed: func() {
			a.emitRouter()
			a.emitModel()
			if a.ctx != nil {
				wr.EventsEmit(a.ctx, "providers", nil)
			}
		},
		ConnectOpenRouter: func() (string, error) {
			r := a.ConnectOpenRouter()
			if !r.OK {
				return "", errors.New(r.Error)
			}
			return r.Model, nil
		},
		AddKey: func(provider, key string) (string, string, error) {
			r := a.AddProvider(provider, key)
			if !r.OK {
				return "", "", errors.New(r.Error)
			}
			return r.Model, r.Note, nil
		},
	})
	s := a.router.Settings()
	// v0.19.1: Argus Cloud answers by default, like Argus Mobile. ArgusRouter only starts when
	// the user turned it on, or their Custom models still go through it.
	if (s.On() && s.AutoStart()) || a.routerInUse() {
		_ = a.router.Start()
	}
	go func() {
		for range time.Tick(15 * time.Second) {
			a.emitRouter()
		}
	}()
}

// routerInUse is true when Custom models answer through ArgusRouter.
func (a *App) routerInUse() bool {
	return !cloudModelsOn() && a.cfg != nil && a.cfg.Default != nil && a.cfg.Default.Provider == config.RouterID
}

func (a *App) stopRouter() {
	if a.router != nil {
		a.router.Stop()
	}
}

// RouterView is what the top-bar button, the tray and the ArgusRouter view show.
type RouterView struct {
	router.Status
	Key       string `json:"key"`
	Dashboard string `json:"dashboard"` // signed-in dashboard address for this run ("" when off)
	Enabled   bool   `json:"enabled"`
	Autostart bool   `json:"autostart"`
	InUse     bool   `json:"in_use"` // Argus's own chats go through ArgusRouter
}

func (a *App) routerView() RouterView {
	if a.router == nil {
		return RouterView{Status: router.Status{Level: "red", Note: brand.NAME + "Router is starting"}}
	}
	s := a.router.Settings()
	v := RouterView{Status: a.router.Status(), Key: s.APIKey, Dashboard: a.router.DashboardURL(""), Enabled: s.On() || a.routerInUse(), Autostart: s.AutoStart()}
	v.InUse = a.cfg != nil && a.cfg.Default != nil && a.cfg.Default.Provider == config.RouterID
	return v
}

func (a *App) emitRouter() {
	if a.ctx == nil || a.router == nil {
		return
	}
	v := a.routerView()
	wr.EventsEmit(a.ctx, "router", v)
	setTrayRouter(v)
}

// RouterInfo is ArgusRouter's status, address and local key.
func (a *App) RouterInfo() RouterView {
	a.waitReady()
	return a.routerView()
}

// RouterDashboardURL is the dashboard address (signed in for this run), optionally at
// one page (home, providers, combos, usage, quota, settings). It starts the server if
// it is off. "" when it can't start.
func (a *App) RouterDashboardURL(page string) string {
	a.waitReady()
	if !a.router.Running() {
		if err := a.router.SetEnabled(true); err != nil {
			return ""
		}
	}
	return a.router.DashboardURL(page)
}

// OpenRouterDashboard opens the dashboard in the system browser.
func (a *App) OpenRouterDashboard() string {
	u := a.RouterDashboardURL("")
	if u == "" {
		return brand.NAME + "Router couldn't start: " + a.router.Status().Note
	}
	a.OpenURL(u)
	return ""
}

// SetRouterOn starts or stops the server (remembered). Returns an error message or "".
func (a *App) SetRouterOn(on bool) string {
	a.waitReady()
	err := a.router.SetEnabled(on)
	a.emitRouter()
	a.emitModel()
	if err != nil {
		return err.Error()
	}
	return ""
}

// RouterPick is one row of the model picker.
type RouterPick struct {
	Group    string `json:"group"`    // argusrouter, combo, provider
	Provider string `json:"provider"` // what SelectModel takes
	Model    string `json:"model"`
	Name     string `json:"name"`
	Desc     string `json:"desc"`
	Current  bool   `json:"current"`
	Tools    bool   `json:"tools"`
}

// pickerPresets are the auto/* presets shown first in the picker (all of them work;
// the full list is under "All ArgusRouter presets").
var pickerPresets = []string{"auto/best-free", "auto", "auto/best-coding", "auto/best-fast", "auto/best-reasoning", "auto/best-vision", "auto/chaos"}

// RouterPicker lists what the model picker and /model offer: ArgusRouter presets and
// the user's combos first, then each ready provider with its model.
func (a *App) RouterPicker() []RouterPick {
	a.waitReady()
	sel := a.selection()
	cur := func(p, m string) bool { return !sel.Auto && sel.Provider == p && sel.Model == m }
	var out []RouterPick
	running := a.router != nil && a.router.Running()
	note := ""
	if !running {
		note = " (" + brand.NAME + "Router is off: picking turns it on)"
	}
	shown := map[string]bool{}
	for _, id := range pickerPresets {
		shown[id] = true
	}
	for _, p := range router.AutoPresets {
		g := "argusrouter"
		if !shown[p.ID] {
			g = "preset"
		}
		out = append(out, RouterPick{Group: g, Provider: config.RouterID, Model: p.ID, Name: brand.NAME + "Router · " + p.ID, Desc: p.Desc + note, Current: cur(config.RouterID, p.ID), Tools: true})
	}
	if a.router != nil {
		for _, c := range a.router.Settings().Combos {
			out = append(out, RouterPick{Group: "combo", Provider: config.RouterID, Model: c.ID, Name: brand.NAME + "Router · " + c.Name, Desc: "Your combo · " + c.Strategy + note, Current: cur(config.RouterID, c.ID), Tools: true})
		}
	}
	for _, p := range a.providerViews() {
		if p.ID == config.KeylessID || p.ID == config.RouterID || !p.Ready {
			continue
		}
		out = append(out, RouterPick{Group: "provider", Provider: p.ID, Model: p.Model, Name: p.Short, Desc: p.Model, Current: !sel.Auto && sel.Provider == p.ID, Tools: llm.CanUseTools(p.ID, p.Model)})
	}
	return out
}

// selectRouter makes an ArgusRouter preset or combo the default (starting the server
// if needed). ArgusRouter never joins the chain: it routes across the chain itself.
func (a *App) selectRouter(model string) string {
	if model == "" {
		model = "auto/best-free"
	}
	if !a.router.Running() {
		if err := a.router.SetEnabled(true); err != nil {
			return brand.NAME + "Router couldn't start: " + err.Error()
		}
	}
	a.cfg.Default = &config.Entry{Provider: config.RouterID, Model: strings.TrimSpace(model)}
	_ = a.cfg.Save()
	a.emitModel()
	a.emitRouter()
	return a.modelName()
}
