package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lacksonkapapahue/blaze-desktop/internal/agents"
	"github.com/lacksonkapapahue/blaze-desktop/internal/bgrun"
	"github.com/lacksonkapapahue/blaze-desktop/internal/bgrun/bgruntest"
	"github.com/lacksonkapapahue/blaze-desktop/internal/config"
	"github.com/lacksonkapapahue/blaze-desktop/internal/llm"
)

// v0.19.14 "ack now, work in background, report when done": the app wiring behind its ports
// (runsAPIOverride = a fake gateway, eventTap = the chat view, notifyTap = desktop notifications).

type deskTap struct {
	mu    sync.Mutex
	evs   []string
	notes []string
	runs  []string // 0.19.16 run_event (working pill / delivery status), kept apart from the chat's events
}

func (d *deskTap) on(ev string, v any) {
	if ev == "topic" || ev == "live" {
		return // chat bookkeeping, not what the chat shows
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if ev == "run_event" {
		b, _ := json.Marshal(v)
		d.runs = append(d.runs, string(b))
		return
	}
	b, _ := json.Marshal(v)
	d.evs = append(d.evs, ev+" "+string(b))
}

func (d *deskTap) note(_, text string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.notes = append(d.notes, text)
}

func (d *deskTap) snap() ([]string, []string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]string(nil), d.evs...), append([]string(nil), d.notes...)
}

func (d *deskTap) find(prefix string) []string {
	evs, _ := d.snap()
	var out []string
	for _, e := range evs {
		if strings.HasPrefix(e, prefix) {
			out = append(out, e)
		}
	}
	return out
}

func (d *deskTap) waitFor(t *testing.T, prefix string) string {
	t.Helper()
	for i := 0; i < 500; i++ {
		if f := d.find(prefix); len(f) > 0 {
			return f[0]
		}
		time.Sleep(10 * time.Millisecond)
	}
	evs, _ := d.snap()
	t.Fatalf("no %q event; got %v", prefix, evs)
	return ""
}

type deskEnv struct {
	a     *App
	fake  *bgruntest.Fake
	tap   *deskTap
	tools []string
}

func deskSetup(t *testing.T, cloud, focused bool) *deskEnv {
	t.Helper()
	a := testApp(t)
	a.pv = &preview{}
	a.prefs.Folder = t.TempDir()
	a.newEnv()
	a.sh.visible = true
	d := &deskEnv{a: a, fake: &bgruntest.Fake{}, tap: &deskTap{}}
	oldAPI, oldOn, oldTap, oldNote, oldDev, oldSleep, oldBg := runsAPIOverride, runsOnOverride, eventTap, notifyTap, deviceToolTap, runsSleep, bgRuns
	runsAPIOverride = d.fake
	runsOnOverride = func() bool { return cloud }
	eventTap, notifyTap = d.tap.on, d.tap.note
	var tmu sync.Mutex
	deviceToolTap = func(_ context.Context, conv, name, args string) string {
		tmu.Lock()
		d.tools = append(d.tools, name+" "+args)
		tmu.Unlock()
		if strings.Contains(args, "fail") {
			return "Error: the scheduler isn't running"
		}
		return "Scheduled: " + args
	}
	runsSleep = func(ctx context.Context, _ time.Duration) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Millisecond):
			return nil
		}
	}
	bgRuns = nil
	winFocused.Store(focused)
	t.Cleanup(func() {
		if bgRuns != nil {
			bgRuns.Wait()
		}
		runsAPIOverride, runsOnOverride, eventTap, notifyTap, deviceToolTap, runsSleep, bgRuns = oldAPI, oldOn, oldTap, oldNote, oldDev, oldSleep, oldBg
		winFocused.Store(true)
	})
	return d
}

// turn is App.turn without the Wails context: the user message, the session start, bgTurn.
func (d *deskEnv) turn(ctx context.Context, text string) (*session, bool) {
	a := d.a
	s := a.screen()
	a.mu.Lock()
	s.hist = append(s.hist, llm.Message{Role: "user", Content: text})
	cctx, cancel := context.WithCancel(ctx)
	a.startSession(s, cancel)
	a.mu.Unlock()
	return s, a.bgTurn(cctx, s, agents.Input{Kind: "user", Text: text})
}

const ackLine = "Got it, setting that reminder for 9:00 tomorrow. I'll confirm when it's set."

func reminderAck(bgrun.CreateReq) *bgrun.Ack {
	return &bgrun.Ack{Intent: "reminder", Line: ackLine}
}

func TestDeskCloudAckThenDone(t *testing.T) {
	d := deskSetup(t, true, true)
	t.Setenv("TZ", "Africa/Lusaka")
	d.fake.AckFor = reminderAck
	d.fake.ScriptFor = func(bgrun.CreateReq) []bgrun.View {
		return []bgrun.View{bgruntest.Running(), bgruntest.NeedsDevice("c1", "schedule_add", `{"at":"9:00"}`), bgruntest.Done("Reminder set for 9:00 tomorrow.", "Reminder set for 9:00 tomorrow.", "reminder")}
	}
	start := time.Now()
	s, ok := d.turn(context.Background(), "remind me to call mum at 9 tomorrow")
	if !ok {
		t.Fatal("a Cloud turn wasn't handled as a run")
	}
	if took := time.Since(start); took > time.Second {
		t.Fatalf("the ack took %v (the turn must end at once)", took)
	}
	// The ack is the turn's bubble and the turn is over (composer free) before the final lands.
	evs, _ := d.tap.snap()
	if len(evs) < 2 || evs[0] != `delta "`+ackLine+`"` || !strings.HasPrefix(evs[1], `done {"ack":true`) {
		t.Fatalf("ack not shown at once: %v", evs)
	}
	if s.running() {
		t.Fatal("the composer is still blocked")
	}
	msg := d.tap.waitFor(t, "blaze_msg")
	if !strings.Contains(msg, "Done ✓ Reminder set for 9:00 tomorrow.") {
		t.Fatalf("final: %s", msg)
	}
	created, _, deleted := d.fake.Snapshot()
	if len(created) != 1 || created[0].TZ != "Africa/Lusaka" || !strings.HasPrefix(created[0].ClientMsgID, bgrun.IDPrefix) {
		t.Fatalf("create: %+v", created)
	}
	bgRuns.Wait()
	if _, _, deleted = d.fake.Snapshot(); len(deleted) != 1 {
		t.Fatalf("delivered run not deleted: %v", deleted)
	}
	// The ack never says done; the chat keeps the ack and then the final.
	a := d.a
	a.mu.Lock()
	var said []string
	for _, m := range s.hist {
		if m.Role == "assistant" {
			said = append(said, m.Content)
		}
	}
	a.mu.Unlock()
	if len(said) != 2 || said[0] != ackLine || bgrun.ClaimsDone(said[0]) || !strings.HasPrefix(said[1], "Done ✓") {
		t.Fatalf("history: %q", said)
	}
	if _, notes := d.tap.snap(); len(notes) != 0 {
		t.Fatalf("focused window got notifications: %v", notes)
	}
}

func TestDeskCloudGreetingNoAck(t *testing.T) {
	d := deskSetup(t, true, true)
	// Greetings, chat, questions, look-ups: no run and no ack; the classic streaming turn answers.
	for _, q := range []string{"hi", "thanks!", "what's today's date?", "latest AI news", "What's the weather in Lusaka right now?", "summarize this page for me"} {
		if _, ok := d.turn(context.Background(), q); ok {
			t.Fatalf("%q went the ack/background way", q)
		}
	}
	created, _, _ := d.fake.Snapshot()
	evs, notes := d.tap.snap()
	if len(created) != 0 || len(notes) != 0 {
		t.Fatalf("a greeting or question must start no run: created=%d notes=%v evs=%v", len(created), notes, evs)
	}
	for _, e := range evs {
		if strings.Contains(e, `"ack":true`) || strings.HasPrefix(e, "blaze_msg") || strings.Contains(e, "Done ✓") {
			t.Fatalf("a greeting must get no promise and no Done: %v", evs)
		}
	}
}

func TestDeskCloudNeedsDeviceAnswered(t *testing.T) {
	d := deskSetup(t, true, true)
	d.fake.AckFor = reminderAck
	d.fake.ScriptFor = func(bgrun.CreateReq) []bgrun.View {
		return []bgrun.View{bgruntest.Running(), bgruntest.NeedsDevice("call_9", "schedule_add", `{"at":"Fri 09:00"}`),
			bgruntest.Running(), bgruntest.Done("Reminder set for Fri 9:00.", "Reminder set for Fri 9:00.", "reminder")}
	}
	d.turn(context.Background(), "remind me on friday at 9 to pay rent")
	msg := d.tap.waitFor(t, "blaze_msg")
	bgRuns.Wait()
	_, res, _ := d.fake.Snapshot()
	if len(d.tools) != 1 || !strings.HasPrefix(d.tools[0], "schedule_add") || len(res) != 1 {
		t.Fatalf("device call not answered: tools=%v results=%v", d.tools, res)
	}
	for _, r := range res {
		if r[0][0].ToolCallID != "call_9" || !strings.Contains(r[0][0].Content, "Scheduled") {
			t.Fatalf("result: %+v", r)
		}
	}
	if !strings.Contains(msg, "Done ✓ Reminder set for Fri 9:00.") {
		t.Fatalf("final: %s", msg)
	}
}

func TestDeskCloudDeviceFailNeverDone(t *testing.T) {
	d := deskSetup(t, true, false)
	d.fake.AckFor = reminderAck
	d.fake.ScriptFor = func(bgrun.CreateReq) []bgrun.View {
		return []bgrun.View{bgruntest.NeedsDevice("c1", "schedule_add", `{"mode":"fail"}`), bgruntest.Done("Reminder set.", "Reminder set.", "reminder")}
	}
	d.turn(context.Background(), "remind me at 9")
	msg := d.tap.waitFor(t, "blaze_msg")
	bgRuns.Wait()
	_, notes := d.tap.snap()
	if strings.Contains(msg, "Done ✓") || len(notes) != 1 || strings.HasPrefix(notes[0], "Done") {
		t.Fatalf("a failed device tool read as done: %s %v", msg, notes)
	}
}

func TestDeskCloudFailedPath(t *testing.T) {
	d := deskSetup(t, true, true)
	d.fake.AckFor = reminderAck
	d.fake.ScriptFor = func(bgrun.CreateReq) []bgrun.View {
		return []bgrun.View{bgruntest.Running(), bgruntest.Failed("models_busy", "Every free model is busy right now.", "Try again in a minute.")}
	}
	d.turn(context.Background(), "remind me at 9")
	msg := d.tap.waitFor(t, "blaze_msg")
	if !strings.Contains(msg, "Every free model is busy right now.") || !strings.Contains(msg, "Try again in a minute.") ||
		strings.Contains(msg, "Done") || !strings.Contains(msg, `"kind":"failed"`) {
		t.Fatalf("failed final: %s", msg)
	}
}

func TestDeskSingleNotificationWhenUnfocused(t *testing.T) {
	d := deskSetup(t, true, false)
	d.fake.AckFor = reminderAck
	d.fake.ScriptFor = func(bgrun.CreateReq) []bgrun.View {
		return []bgrun.View{bgruntest.Running(), bgruntest.NeedsDevice("c1", "schedule_add", `{}`), bgruntest.Running(), bgruntest.Done("Reminder set for Fri 9:00.", "Reminder set for Fri 9:00.", "reminder")}
	}
	d.turn(context.Background(), "remind me friday 9am")
	d.tap.waitFor(t, "blaze_msg")
	bgRuns.Wait()
	// A second resume / duplicate follow must not notify again.
	bgRuns.Follow(context.Background(), bgrun.Entry{RunID: "run_fake1", Conv: mainConv, Action: true}, 0)
	bgRuns.Wait()
	_, notes := d.tap.snap()
	if len(notes) != 1 || notes[0] != "Done, reminder set for Fri 9:00" {
		t.Fatalf("want exactly one notification, got %v", notes)
	}
}

func TestDeskNoNotificationWhenFocused(t *testing.T) {
	d := deskSetup(t, true, true)
	d.fake.AckFor = reminderAck
	d.fake.ScriptFor = func(bgrun.CreateReq) []bgrun.View {
		return []bgrun.View{bgruntest.Done("Reminder set for Fri 9:00.", "Reminder set for Fri 9:00.", "reminder")}
	}
	d.turn(context.Background(), "remind me friday 9am")
	d.tap.waitFor(t, "blaze_msg")
	bgRuns.Wait()
	if _, notes := d.tap.snap(); len(notes) != 0 {
		t.Fatalf("focused window got %v", notes)
	}
}

func TestDeskResumePendingRuns(t *testing.T) {
	d := deskSetup(t, true, false)
	a := d.a
	// Left from the last session: a finished run and one waiting for this computer.
	d.fake.Seed("run_old1", "desk-old1", bgruntest.Done("I couldn't tell which day you meant. Today or tomorrow?", "", "reminder"))
	d.fake.Seed("run_old2", "desk-old2", bgruntest.NeedsDevice("c7", "schedule_add", `{"at":"8:00"}`), bgruntest.Done("Alarm set for 8:00.", "Alarm set for 8:00.", "alarm"))
	d.fake.Seed("run_phone", "mob-1", bgruntest.Done("phone", "", ""))
	m := a.ensureBgRuns()
	s := a.screen()
	a.mu.Lock()
	s.id = mainConv
	a.mu.Unlock()
	a.saveConv(s)
	m.Track(bgrun.Entry{RunID: "run_old1", ClientMsgID: "desk-old1", Conv: mainConv, Action: true, Created: time.Now()})
	m.Track(bgrun.Entry{RunID: "run_old2", ClientMsgID: "desk-old2", Conv: mainConv, Action: true, Created: time.Now()})
	a.resumeRuns(0)
	m.Wait()
	msgs := d.tap.find("blaze_msg")
	if len(msgs) != 2 {
		t.Fatalf("resume delivered %d finals: %v", len(msgs), msgs)
	}
	_, res, deleted := d.fake.Snapshot()
	if len(res["run_old2"]) != 1 || res["run_phone"] != nil || len(deleted) != 2 {
		t.Fatalf("results %v deleted %v", res, deleted)
	}
	if len(m.Store.All()) != 0 {
		t.Fatalf("pending left: %v", m.Store.All())
	}
	_, notes := d.tap.snap()
	if len(notes) != 2 {
		t.Fatalf("one notification per final while unfocused, got %v", notes)
	}
	// The finals are in the chat.
	a.mu.Lock()
	n := 0
	for _, m := range s.hist {
		if m.Role == "assistant" && strings.HasPrefix(m.Content, "Done ✓") {
			n++
		}
	}
	a.mu.Unlock()
	if n != 1 { // the talk-only final is shown as is, the device one confirmed
		t.Fatalf("confirmed finals in chat: %d", n)
	}
}

func TestDeskNewMessageMovesRunToBackground(t *testing.T) {
	d := deskSetup(t, true, true)
	noLocalAck(t) // the gateway-sent-no-ack foreground follow
	// No ack from the gateway, slow: the user sends another message while it works.
	views := []bgrun.View{}
	for i := 0; i < 40; i++ {
		views = append(views, bgruntest.Running())
	}
	views = append(views, bgruntest.Done("Paris is the capital of France.", "Paris is the capital of France.", "chat"))
	d.fake.ScriptFor = func(bgrun.CreateReq) []bgrun.View { return views }
	a := d.a
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan bool)
	go func() { _, ok := d.turn(ctx, "create a file capital.txt with the capital of france"); done <- ok }()
	time.Sleep(15 * time.Millisecond)
	s := a.screen()
	a.mu.Lock()
	s.detach = true // what SendWith does before Stop
	a.mu.Unlock()
	cancel()
	<-done
	msg := d.tap.waitFor(t, "blaze_msg")
	if !strings.Contains(msg, "Paris is the capital of France.") {
		t.Fatalf("final: %s", msg)
	}
}

func TestDeskCloudOlderGatewayFallsBack(t *testing.T) {
	d := deskSetup(t, true, true)
	d.fake.CreateErr = &bgrun.HTTPError{Code: 404, Body: "not_found"}
	if _, ok := d.turn(context.Background(), "remind me at 9"); ok {
		t.Fatal("a gateway without runs must fall back to the classic loop")
	}
}

func TestDeskLocalAckAndBackground(t *testing.T) {
	d := deskSetup(t, false, false)
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "marker.txt"), []byte("x"), 0o600)
	arg, _ := json.Marshal(map[string]string{"path": dir})
	var tc llm.ToolCall
	tc.ID, tc.Type = "c1", "function"
	tc.Function.Name, tc.Function.Arguments = "file_list", string(arg)
	sc := &scripted{replies: []*llm.Result{{ToolCalls: []llm.ToolCall{tc}, Finish: "tool_calls"}, {Text: "There is one file, marker.txt.", Finish: "stop"}}}
	old := completeTurn
	completeTurn = sc.complete
	t.Cleanup(func() { completeTurn = old })
	s, ok := d.turn(context.Background(), "create a listing of the files in "+dir)
	if !ok {
		t.Fatal("a local action wasn't acked")
	}
	evs, _ := d.tap.snap()
	if len(evs) < 2 || !strings.HasPrefix(evs[0], `delta "On it.`) || !strings.HasPrefix(evs[1], `done {"ack":true`) || bgrun.ClaimsDone(evs[0]) {
		t.Fatalf("local ack: %v", evs)
	}
	if s.running() {
		t.Fatal("composer blocked")
	}
	msg := d.tap.waitFor(t, "blaze_msg")
	if !strings.Contains(msg, "Done ✓ There is one file, marker.txt.") {
		t.Fatalf("local final: %s", msg)
	}
	// No "working" lines reached the chat from the background turn.
	for _, e := range d.tap.find("") {
		if strings.HasPrefix(e, "status") || strings.HasPrefix(e, "tool ") || strings.HasPrefix(e, "thinking") {
			t.Fatalf("background turn leaked progress: %s", e)
		}
	}
	_, notes := d.tap.snap()
	if len(notes) != 1 {
		t.Fatalf("unfocused: want one notification, got %v", notes)
	}
	// Local greeting: no ack, the classic path answers.
	if _, ok := d.turn(context.Background(), "hi"); ok {
		t.Fatal("a local greeting got an ack")
	}
}

func TestDeskLocalFailedPath(t *testing.T) {
	d := deskSetup(t, false, true)
	old := completeTurn
	completeTurn = func(context.Context, *config.Config, []llm.Message, []llm.Tool, llm.Opts) (*llm.Result, error) {
		return nil, &llm.HTTPError{Code: 400, Body: "bad request"}
	}
	t.Cleanup(func() { completeTurn = old })
	d.turn(context.Background(), "create a file notes.txt with hello")
	msg := d.tap.waitFor(t, "blaze_msg")
	if !strings.Contains(msg, "Couldn't finish that.") || strings.Contains(msg, "Done ✓") || !strings.Contains(msg, `"kind":"failed"`) {
		t.Fatalf("local failed final: %s", msg)
	}
}

func TestOSTimeZone(t *testing.T) {
	t.Setenv("TZ", "Africa/Lusaka")
	if tz := osTimeZone(); tz != "Africa/Lusaka" {
		t.Fatalf("tz %q", tz)
	}
	for _, bad := range []string{"", "Local", "../../etc", "a b"} {
		if validTZ(bad) {
			t.Fatalf("%q accepted", bad)
		}
	}
}

// A resume (start-up, refocus) while a no-ack turn is watching its run must leave that run to the
// turn: the reply lands in the turn's own bubble, the turn ends with done, nothing lands twice.
// (Live 2026-10-23: the start-up resume took the run, so the turn never ended.)
func TestDeskResumeLeavesForegroundRunAlone(t *testing.T) {
	d := deskSetup(t, true, true)
	noLocalAck(t) // the gateway-sent-no-ack foreground follow
	d.fake.ScriptFor = func(bgrun.CreateReq) []bgrun.View {
		vs := []bgrun.View{}
		for i := 0; i < 40; i++ {
			vs = append(vs, bgruntest.Running())
		}
		return append(vs, bgruntest.Done("It's Friday 23 October.", "It's Friday 23 October.", "chat"))
	}
	stop := make(chan struct{})
	go func() {
		for {
			select {
			case <-stop:
				return
			default:
				d.a.resumeRuns(0)
				time.Sleep(time.Millisecond)
			}
		}
	}()
	_, ok := d.turn(context.Background(), "create a file today.txt with today's date")
	close(stop)
	if !ok {
		t.Fatal("not handled")
	}
	if bgRuns != nil {
		bgRuns.Wait()
	}
	if len(d.tap.find("done ")) != 1 || len(d.tap.find(`delta "It's Friday 23 October."`)) != 1 || len(d.tap.find("blaze_msg")) != 0 {
		evs, _ := d.tap.snap()
		t.Fatalf("the turn's own reply must end the turn once: %v", evs)
	}
}

// TestBgActionRouting: only "do something" turns take the ack-and-background way; chat,
// questions, look-ups and summaries keep the normal streaming turn.
func TestBgActionRouting(t *testing.T) {
	bg := []string{
		"create a file called notes.txt with hello in it",
		"remind me to call mum at 5pm",
		"run ls -la in my project",
		"please delete old.log",
		"remember that my favourite colour is teal",
		"send an email to sam saying hi",
		"fix the failing tests",
		"Read notes-1.txt in this folder, find the magic word, and create out-1.txt containing only that word.",
		"Now edit out-1.txt: change the word to zephyr-edited.",
		"Run this command and tell me its exact output: echo blaze-$((42+1))-ok",
		"look up the weather in Lusaka and save it to weather.txt",
		"Create a file called ack-1.txt in this folder containing the word ready.",
		"Make a CSV report of sales-1.csv in this folder: total amount by region, with a bar chart.",
		"generate a bar chart of sales.csv by region",
	}
	stream := []string{
		"hi", "thanks!", "good morning",
		"what is a goroutine?",
		"how do I create a file in Go?",
		"tell me a joke",
		"what do you remember about me?",
		"what have we worked on together?",
		"read notes.txt",
		"list the files in this folder",
		"show me the readme",
		"search the web for the latest Go release",
		"what's the weather in Lusaka today?",
		"summarize this article for me",
		"do you remember my favourite colour?",
		"search for how to run Go tests",
		"latest AI news",
		"find where we create the config file",
	}
	for _, s := range bg {
		if _, ok := bgAction(s); !ok {
			t.Errorf("want background for %q", s)
		}
	}
	for _, s := range stream {
		if in, ok := bgAction(s); ok {
			t.Errorf("want streaming for %q (intent %q)", s, in)
		}
	}
}

func noLocalAck(t *testing.T) {
	cloudLocalAck = false
	t.Cleanup(func() { cloudLocalAck = true })
}

// An action the gateway sends no ack line for still gets one at once (the local line), the
// composer is free, and the final lands as "Done ✓" (live: "Run this command…" sat 9-15 s).
func TestDeskCloudActionWithoutGatewayAckStillAcks(t *testing.T) {
	d := deskSetup(t, true, true)
	d.fake.AckFor = nil
	d.fake.ScriptFor = func(bgrun.CreateReq) []bgrun.View {
		return []bgrun.View{bgruntest.Running(), bgruntest.NeedsDevice("call_1", "run_shell", `{"command":"echo blaze-43-ok"}`),
			bgruntest.Running(), bgruntest.Done("blaze-43-ok", "blaze-43-ok", "search")} // the receipt calls it a search
	}
	start := time.Now()
	s, ok := d.turn(context.Background(), "Run this command and tell me its exact output: echo blaze-$((42+1))-ok")
	if !ok {
		t.Fatal("not handled as a run")
	}
	if took := time.Since(start); took > time.Second {
		t.Fatalf("ack took %v", took)
	}
	evs, _ := d.tap.snap()
	if len(evs) < 2 || !strings.HasPrefix(evs[0], `delta "On it.`) || !strings.HasPrefix(evs[1], `done {"ack":true`) {
		t.Fatalf("no local ack: %v", evs)
	}
	if s.running() {
		t.Fatal("composer blocked")
	}
	msg := d.tap.waitFor(t, "blaze_msg")
	if !strings.Contains(msg, "Done ✓ blaze-43-ok") {
		t.Fatalf("final: %s", msg)
	}
}

// A background final that lands while a newer turn runs goes in before that turn's question.
func TestFinalLandsBeforeRunningTurn(t *testing.T) {
	u1 := llm.Message{Role: "user", Content: "create bg.txt"}
	ack := llm.Message{Role: "assistant", Content: "On it."}
	u2 := llm.Message{Role: "user", Content: "what's 17 times 3?"}
	fin := llm.Message{Role: "assistant", Content: "Done ✓ Created bg.txt."}
	got := insertFinal([]llm.Message{u1, ack, u2}, fin, true)
	if len(got) != 4 || got[2].Content != fin.Content || got[3].Content != u2.Content {
		t.Fatalf("running: %+v", got)
	}
	got = insertFinal([]llm.Message{u1, ack}, fin, false)
	if len(got) != 3 || got[2].Content != fin.Content {
		t.Fatalf("idle: %+v", got)
	}
}

// Ack bubbles stay in the chat but never reach the model.
func TestAcksNotSentToModel(t *testing.T) {
	for _, l := range []string{"On it. I'll confirm when it's done.", "On it. I'll let you know when it's done.",
		"Got it, setting that reminder for 9:00 tomorrow. I'll confirm when it's set.", "On it, looking that up. I'll report back with what I find.",
		"On it. I'll tell you how it went when it's finished."} {
		if !bgrun.IsAckLine(l) {
			t.Errorf("not an ack: %q", l)
		}
	}
	for _, l := range []string{"Done ✓ Created a.txt.", "On it the file says hello.\nMore", "Sure! Paris is the capital of France.", "blaze-43-ok"} {
		if bgrun.IsAckLine(l) {
			t.Errorf("not an ack but dropped: %q", l)
		}
	}
	h := []llm.Message{{Role: "user", Content: "create a.txt"}, {Role: "assistant", Content: "On it. I'll confirm when it's done."},
		{Role: "assistant", Content: "Done ✓ Created a.txt."}, {Role: "user", Content: "now b.txt"}}
	got := dropAcks(h)
	if len(got) != 3 || got[1].Content != "Created a.txt." || len(h) != 4 || h[2].Content != "Done ✓ Created a.txt." {
		t.Fatalf("dropAcks: %+v", got)
	}
	// still running: the task and its ack are both left out
	h = []llm.Message{{Role: "user", Content: "hi"}, {Role: "assistant", Content: "Hey!"}, {Role: "user", Content: "create b.txt"},
		{Role: "assistant", Content: "On it. I'll let you know when it's done."}, {Role: "user", Content: "what's 17 times 3?"}}
	got = dropAcks(h)
	if len(got) != 3 || got[2].Content != "what's 17 times 3?" || got[1].Content != "Hey!" {
		t.Fatalf("pending task: %+v", got)
	}
}
