package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lacksonkapapahue/blaze-desktop/internal/notify"
)

func noteOf(kind string) notify.Note { return notify.Note{Kind: kind, Title: "Blaze", Body: "Done."} }

// v0.19.14: onboarding is two steps (sign in, optional folder), in that order.
func TestOnboardTwoSteps(t *testing.T) {
	a := testApp(t)
	st := a.Onboarding()
	if len(st.Steps) != 2 || st.Steps[0] != "signin" || st.Steps[1] != "folder" {
		t.Fatalf("onboarding steps = %v, want [signin folder]", st.Steps)
	}
	for _, gone := range []string{"welcome", "model", "apps", "perms", "tour", "done"} {
		for _, s := range st.Steps {
			if s == gone {
				t.Fatalf("old step %q is still in onboarding", gone)
			}
		}
	}
}

// A step saved by the old six-step flow (e.g. "tour", 4) resumes inside the new flow.
func TestOnboardStepClamped(t *testing.T) {
	a := testApp(t)
	a.prefs.OnboardStep = 4
	if got := a.Onboarding().Step; got != 1 {
		t.Fatalf("old saved step 4 resumes at %d, want 1", got)
	}
	a.SetOnboardStep(9)
	if a.prefs.OnboardStep != 1 {
		t.Fatalf("SetOnboardStep(9) saved %d, want 1", a.prefs.OnboardStep)
	}
	a.SetOnboardStep(-3)
	if a.prefs.OnboardStep != 0 {
		t.Fatalf("SetOnboardStep(-3) saved %d, want 0", a.prefs.OnboardStep)
	}
	a.SetOnboardStep(1)
	b, _ := os.ReadFile(prefsFile())
	var p Prefs
	_ = json.Unmarshal(b, &p)
	if p.OnboardStep != 1 {
		t.Fatalf("step not saved: %d", p.OnboardStep)
	}
	a.FinishOnboarding()
	if !a.cfg.SetupDone || !a.prefs.OnboardSkipped || a.prefs.OnboardStep != 0 {
		t.Fatal("finishing onboarding didn't mark setup done")
	}
}

// Folder access is asked only when a message needs a folder and none was picked.
func TestPermFolderAskedJustInTime(t *testing.T) {
	a := testApp(t)
	home := t.TempDir()
	a.info.Home = home
	a.prefs.Folder = home // startup's default when no folder was picked
	if p := a.PermAsk("folder", "what's the weather in Lusaka?"); p.Ask {
		t.Fatal("asked for a folder for a weather question")
	}
	p := a.PermAsk("folder", "Run the tests here and fix whatever fails.")
	if !p.Ask || p.Reason == "" || p.Fallback == "" || strings.Contains(p.Reason, "\n") {
		t.Fatalf("folder ask = %+v, want one-line reason and a fallback", p)
	}
	for _, s := range []string{"What's in this folder?", "summarise my code", "git status please", "look at the repo"} {
		if !a.PermAsk("folder", s).Ask {
			t.Fatalf("no folder ask for %q", s)
		}
	}
	// A picked folder: no ask.
	proj := filepath.Join(home, "code")
	_ = os.MkdirAll(proj, 0o700)
	a.prefs.Folder = proj
	if a.PermAsk("folder", "What's in this folder?").Ask {
		t.Fatal("asked again with a folder picked")
	}
	// "Not now" sticks, with an honest fallback line.
	a.prefs.Folder = home
	r := a.PermAnswer("folder", false)
	if r.Allowed || !strings.Contains(r.Line, "home folder") {
		t.Fatalf("decline = %+v", r)
	}
	if a.PermAsk("folder", "What's in this folder?").Ask {
		t.Fatal("asked again after Not now")
	}
	if st := a.PermState(); st.FolderChosen || st.Folder != home {
		t.Fatalf("state = %+v, want the home folder, not chosen", st)
	}
}

// Notifications are asked once, at the first long task; "no" stops every notification
// except the Settings test, and the answer is saved.
func TestPermNotifyGate(t *testing.T) {
	a := testApp(t)
	if !a.PermAsk("notify", "").Ask {
		t.Fatal("notifications not asked at first need")
	}
	r := a.PermAnswer("notify", false)
	if r.Allowed || r.Line == "" {
		t.Fatalf("decline = %+v", r)
	}
	if a.PermAsk("notify", "").Ask {
		t.Fatal("asked again after no")
	}
	if !notifyDenied(a.prefs, "task") || !notifyDenied(a.prefs, "job") || notifyDenied(a.prefs, "test") {
		t.Fatal("deny gate wrong")
	}
	if _, err := a.send(noteOf("task")); err == nil || !strings.Contains(err.Error(), "Chat") {
		t.Fatalf("send after deny = %v, want an honest error", err)
	}
	b, _ := os.ReadFile(prefsFile())
	var p Prefs
	_ = json.Unmarshal(b, &p)
	if p.Perms.Notify != "deny" {
		t.Fatalf("answer not saved: %q", p.Perms.Notify)
	}
	a.prefs.Perms.Notify = ""
	a.prefs.Notify.Off = true
	if a.PermAsk("notify", "").Ask {
		t.Fatal("asked although notifications are off in Settings")
	}
}

// No permission prompt at launch: macOS's notification probe (which prompts) waits for the
// first real notification.
func TestNoPromptAtStartup(t *testing.T) {
	if probeAtStartup("darwin") {
		t.Fatal("macOS would be asked for notification permission at launch")
	}
	if !probeAtStartup("linux") || !probeAtStartup("windows") {
		t.Fatal("silent probes should still run at startup")
	}
}

// Confirm cards carry a one-line reason, and a decline says plainly that nothing was done.
func TestPermReasonOneLine(t *testing.T) {
	for _, title := range []string{"Run a risky command?", "Run risky code?", "Run a risky git command?", "Delete?", "Write a file outside this project?", "Install ripgrep?", "Schedule this?", "Send through your phone?"} {
		why := permReason(title, "x")
		if why == "" || strings.Contains(why, "\n") || len(why) > 120 {
			t.Fatalf("reason for %q = %q", title, why)
		}
		if permDeclined(title) == "" {
			t.Fatalf("no declined line for %q", title)
		}
	}
	if permReason("Something else", "") != "" {
		t.Fatal("made up a reason")
	}
}

// The UI draws the same two steps Go reports, and loads the just-in-time asks.
func TestOnboardStepsMatchUI(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("frontend", "dist", "onboard.js"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `const STEPS = [["signin", "Sign in"], ["folder", "Folder"]];`) {
		t.Fatal("onboard.js steps differ from onboardSteps")
	}
	for _, gone := range []string{"function perms(", "function tour(", "function apps(", "OnboardNotifyTest()"} {
		if strings.Contains(string(b), gone) {
			t.Fatalf("onboard.js still has %s: setup must not ask for permissions up front", gone)
		}
	}
	ix, _ := os.ReadFile(filepath.Join("frontend", "dist", "index.html"))
	if !strings.Contains(string(ix), `<script src="perms.js"></script>`) || !strings.Contains(string(ix), `href="perms.css"`) {
		t.Fatal("index.html doesn't load perms.js / perms.css")
	}
	if strings.Index(string(ix), `src="perms.js"`) < strings.Index(string(ix), `src="app.js"`) {
		t.Fatal("perms.js must load after app.js (it wraps submit)")
	}
}
