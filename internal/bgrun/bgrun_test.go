package bgrun_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lacksonkapapahue/argus-desktop/internal/bgrun"
	"github.com/lacksonkapapahue/argus-desktop/internal/bgrun/bgruntest"
)

func noSleep(ctx context.Context, _ time.Duration) error { return ctx.Err() }

func follower(f *bgruntest.Fake, dev bgrun.Device) *bgrun.Follower {
	return &bgrun.Follower{API: f, Device: dev, Sleep: noSleep}
}

const gwLine = "Got it, setting that reminder for 9:00 tomorrow. I'll confirm when it's set."

func TestAckShownForActionNotForGreeting(t *testing.T) {
	if got := bgrun.AckLine(&bgrun.Ack{Intent: "reminder", Line: gwLine}); got != gwLine {
		t.Fatalf("action ack not shown: %q", got)
	}
	if got := bgrun.AckLine(nil); got != "" {
		t.Fatalf("greeting (ack null) must show no promise: %q", got)
	}
	for _, g := range []string{"hi", "thanks!", "hello argus"} {
		if _, l := bgrun.LocalAck(g, false); l != "" {
			t.Fatalf("local greeting %q got an ack %q", g, l)
		}
	}
	if in, l := bgrun.LocalAck("remind me to call mum at 5pm", true); in != "reminder" || l == "" {
		t.Fatalf("local reminder ack: %q %q", in, l)
	}
}

func TestAckNeverSaysDone(t *testing.T) {
	for _, l := range []string{"Done ✓ reminder set for 9:00", "Done! Reminder set.", "Your reminder is set for 9:00.", "All set, I've sent it.", "Reminder has been scheduled."} {
		if got := bgrun.AckLine(&bgrun.Ack{Intent: "reminder", Line: l}); got != "" {
			t.Fatalf("ack claiming completion was shown: %q", got)
		}
	}
	// Every local line is a promise, never a claim.
	for _, q := range []string{"remind me at 9", "set an alarm for 7", "start a 10 minute timer", "schedule a meeting", "send the report to Ana",
		"search for flights", "summarize this", "fix the failing tests", "create a file notes.txt", "do the thing"} {
		_, l := bgrun.LocalAck(q, true)
		if l == "" || bgrun.ClaimsDone(l) {
			t.Fatalf("local ack for %q claims done or is empty: %q", q, l)
		}
	}
	if bgrun.ClaimsDone(gwLine) {
		t.Fatal("the gateway's own template must pass the guard")
	}
}

func TestFollowDoneShowsReceipt(t *testing.T) {
	f := &bgruntest.Fake{}
	f.Seed("r1", "desk-1", bgruntest.Running(), bgruntest.NeedsDevice("c1", "schedule_add", `{}`), bgruntest.Done("Reminder set for 9:00 tomorrow. I'll ping you then.", "Reminder set for 9:00 tomorrow.", "reminder"))
	okDev := func(context.Context, bgrun.ToolCall) (string, bool) { return "Scheduled", true }
	fin, err := follower(f, okDev).Follow(context.Background(), "r1", 0, true)
	if err != nil || fin.Kind != bgrun.KindDone {
		t.Fatalf("follow: %v %+v", err, fin)
	}
	msg := fin.Message()
	if !strings.HasPrefix(msg, "Done ✓ Reminder set for 9:00 tomorrow.") || strings.Count(msg, "Reminder set") != 1 {
		t.Fatalf("done message: %q", msg)
	}
	if n := fin.Notification(); n != "Done, reminder set for 9:00 tomorrow" {
		t.Fatalf("notification: %q", n)
	}
	// A chat reply (no ack) is just the reply: no "Done ✓".
	f.Seed("r2", "desk-2", bgruntest.Done("Hi Ananya! What can I do?", "Hi Ananya!", "chat"))
	fin, _ = follower(f, nil).Follow(context.Background(), "r2", 0, false)
	if fin.Message() != "Hi Ananya! What can I do?" {
		t.Fatalf("chat reply changed: %q", fin.Message())
	}
}

func TestNeedsDeviceAnsweredThenDone(t *testing.T) {
	f := &bgruntest.Fake{}
	f.Seed("r1", "desk-1", bgruntest.Running(), bgruntest.NeedsDevice("call_1", "schedule_add", `{"at":"09:00"}`),
		bgruntest.Running(), bgruntest.Done("Reminder set for Fri 9:00.", "Reminder set for Fri 9:00.", "reminder"))
	var ran []string
	dev := func(_ context.Context, c bgrun.ToolCall) (string, bool) {
		ran = append(ran, c.Function.Name+" "+c.Function.Arguments)
		return "Scheduled for Fri 09:00", true
	}
	fin, err := follower(f, dev).Follow(context.Background(), "r1", 0, true)
	if err != nil || fin.Kind != bgrun.KindDone || !fin.DeviceRan || !fin.DeviceOK {
		t.Fatalf("follow: %v %+v", err, fin)
	}
	_, res, _ := f.Snapshot()
	if len(ran) != 1 || len(res["r1"]) != 1 || res["r1"][0][0].ToolCallID != "call_1" || res["r1"][0][0].Content != "Scheduled for Fri 09:00" {
		t.Fatalf("tool call not answered once: ran=%v results=%v", ran, res)
	}
	if !fin.Confirmed() || !strings.HasPrefix(fin.Message(), "Done ✓") {
		t.Fatalf("not confirmed: %q", fin.Message())
	}
}

func TestDeviceToolFailedNeverConfirms(t *testing.T) {
	f := &bgruntest.Fake{}
	f.Seed("r1", "desk-1", bgruntest.NeedsDevice("c1", "schedule_add", `{}`), bgruntest.Done("Reminder set for 9:00.", "Reminder set for 9:00.", "reminder"))
	dev := func(context.Context, bgrun.ToolCall) (string, bool) { return "Error: no scheduler", false }
	fin, _ := follower(f, dev).Follow(context.Background(), "r1", 0, true)
	if fin.Confirmed() || strings.Contains(fin.Message(), "Done ✓") || !strings.Contains(fin.Message(), bgrun.DeviceFailedLine) {
		t.Fatalf("a failed device tool must never read as done: %q", fin.Message())
	}
	if strings.HasPrefix(fin.Notification(), "Done") {
		t.Fatalf("notification claims done: %q", fin.Notification())
	}
}

func TestFailedPathIsHonest(t *testing.T) {
	f := &bgruntest.Fake{}
	f.Seed("r1", "desk-1", bgruntest.Running(), bgruntest.Failed("models_busy", "Every free model is busy right now.", "Try again in a minute."))
	fin, err := follower(f, nil).Follow(context.Background(), "r1", 0, true)
	if err != nil || fin.Kind != bgrun.KindFailed || fin.Code != "models_busy" {
		t.Fatalf("follow: %v %+v", err, fin)
	}
	msg := fin.Message()
	if !strings.Contains(msg, "Every free model is busy right now.") || !strings.Contains(msg, "Try again in a minute.") || strings.Contains(msg, "Done") {
		t.Fatalf("failed message: %q", msg)
	}
	if !strings.HasPrefix(fin.Notification(), "Couldn't finish that: every free model is busy") {
		t.Fatalf("failed notification: %q", fin.Notification())
	}
	// A run that vanished is a failure too, never a silent success.
	fin, _ = follower(f, nil).Follow(context.Background(), "nope", 0, true)
	if fin.Kind != bgrun.KindFailed {
		t.Fatalf("missing run: %+v", fin)
	}
}

func TestFollowStopsOnContextAndLeavesRun(t *testing.T) {
	f := &bgruntest.Fake{}
	f.Seed("r1", "desk-1", bgruntest.Running())
	ctx, cancel := context.WithCancel(context.Background())
	fl := &bgrun.Follower{API: f, Sleep: func(ctx context.Context, _ time.Duration) error { cancel(); return ctx.Err() }}
	if _, err := fl.Follow(ctx, "r1", 0, true); err == nil {
		t.Fatal("a cancelled follow must return the context error")
	}
}

func TestNotifierOnlyWhenUnfocusedAndOnce(t *testing.T) {
	var sent []string
	focused := true
	n := &bgrun.Notifier{Focused: func() bool { return focused }, Send: func(_, text string) { sent = append(sent, text) }}
	fin := bgrun.Final{Kind: bgrun.KindDone, Action: true, DeviceRan: true, DeviceOK: true, Intent: "reminder", Reply: "Reminder set for Fri 9:00.", Receipt: &bgrun.Receipt{Summary: "Reminder set for Fri 9:00."}}
	if n.Land("r1", "main", fin) || len(sent) != 0 {
		t.Fatalf("focused window got a notification: %v", sent)
	}
	focused = false
	n.Land("r2", "main", fin)
	n.Land("r2", "main", fin)
	if len(sent) != 1 || sent[0] != "Done, reminder set for Fri 9:00" {
		t.Fatalf("want exactly one notification, got %v", sent)
	}
}

type deliveries struct {
	mu  sync.Mutex
	got map[string][]string
}

func (d *deliveries) add(e bgrun.Entry, f bgrun.Final) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.got == nil {
		d.got = map[string][]string{}
	}
	d.got[e.RunID] = append(d.got[e.RunID], f.Message())
}

func TestResumePicksUpPendingRuns(t *testing.T) {
	f := &bgruntest.Fake{}
	// Before the restart: one run finished while Argus was closed, one waits for this computer.
	f.Seed("r_done", "desk-a", bgruntest.Done("Reminder set for 9:00.", "Reminder set for 9:00.", "reminder"))
	f.Seed("r_dev", "desk-b", bgruntest.NeedsDevice("c1", "file_write", `{"path":"a.txt"}`), bgruntest.Done("Saved a.txt.", "Saved a.txt.", "write"))
	f.Seed("r_phone", "mob-c", bgruntest.Done("phone run", "", "")) // another device's run: never touched
	store := &bgrun.Store{Path: filepath.Join(t.TempDir(), "runs.json")}
	store.Add(bgrun.Entry{RunID: "r_done", ClientMsgID: "desk-a", Conv: "main", Action: true, Created: time.Now()})
	store.Add(bgrun.Entry{RunID: "r_dev", ClientMsgID: "desk-b", Conv: "main", Action: true, Created: time.Now()})
	var notes []string
	var nmu sync.Mutex
	d := &deliveries{}
	devCalls := 0
	m := &bgrun.Manager{API: f, Store: store, Deliver: d.add,
		Follower: func(bgrun.Entry) *bgrun.Follower {
			return follower(f, func(context.Context, bgrun.ToolCall) (string, bool) { devCalls++; return "Wrote a.txt", true })
		},
		Notifier: &bgrun.Notifier{Focused: func() bool { return false }, Send: func(_, s string) { nmu.Lock(); notes = append(notes, s); nmu.Unlock() }}}
	n, err := m.Resume(context.Background())
	m.Wait()
	if err != nil || n != 2 {
		t.Fatalf("resume picked up %d (%v)", n, err)
	}
	if len(d.got["r_done"]) != 1 || len(d.got["r_dev"]) != 1 || d.got["r_phone"] != nil {
		t.Fatalf("deliveries: %v", d.got)
	}
	if devCalls != 1 || !strings.HasPrefix(d.got["r_dev"][0], "Done ✓") {
		t.Fatalf("needs_device not answered on resume: calls=%d %v", devCalls, d.got)
	}
	if len(store.All()) != 0 {
		t.Fatalf("store not cleared: %v", store.All())
	}
	_, _, deleted := f.Snapshot()
	if len(deleted) != 2 || len(notes) != 2 {
		t.Fatalf("deleted %v notes %v", deleted, notes)
	}
	// A second resume (or a duplicate follow) delivers nothing again.
	if m.Follow(context.Background(), bgrun.Entry{RunID: "r_done"}, 0) {
		t.Fatal("a delivered run was followed again")
	}
}

func TestClientMsgIDShape(t *testing.T) {
	id := bgrun.NewClientMsgID()
	if !strings.HasPrefix(id, bgrun.IDPrefix) || len(id) < 4 || len(id) > 128 || strings.ContainsAny(id, " /") {
		t.Fatalf("bad client_msg_id %q", id)
	}
}

// Live smoke 2026-10-23: the gateway's message event carries {role, content}; a strict decode of
// it lost every run after the reply. A real 0.8.38 view must decode, with the error string read.
func TestViewDecodesGatewayEvents(t *testing.T) {
	raw := `{"id":"run_x","status":"done","events":[{"seq":2,"type":"ack","intent":"reminder","line":"Got it."},
{"seq":5,"type":"message","step":0,"message":{"role":"assistant","content":"Hi!"}},
{"seq":6,"type":"done","reply":"Hi!","provider":"p","model":"m","receipt":{"intent":"chat","summary":"Hi!"}},
{"seq":7,"type":"error","code":"x","message":"plain text"}],"next_after":7,"reply":"Hi!"}`
	var v bgrun.View
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(v.Events) != 4 || v.Events[3].MessageText() != "plain text" || v.Events[1].MessageText() != "" || v.Events[2].Receipt.Summary != "Hi!" {
		t.Fatalf("events: %+v", v.Events)
	}
}

// Live smoke 2026-10-23 (run_62aa04b10f99c9cc37a337c2): a reminder run ended "done" with the model
// asking for the date and no tool run. That is never a "Done ✓"; nor is a search answer.
func TestDoneWithoutDeviceToolIsNotConfirmed(t *testing.T) {
	f := &bgruntest.Fake{}
	q := "I don't have today's date, so I can't set this reliably. What date is it?"
	f.Seed("r1", "desk-1", bgruntest.Running(), bgruntest.Done(q, q, "reminder"))
	fin, _ := follower(f, nil).Follow(context.Background(), "r1", 0, true)
	if fin.Confirmed() || strings.Contains(fin.Message(), "Done") || strings.HasPrefix(fin.Notification(), "Done") {
		t.Fatalf("talk-only reminder read as done: %q / %q", fin.Message(), fin.Notification())
	}
	if fin.Message() != q {
		t.Fatalf("reply changed: %q", fin.Message())
	}
	f.Seed("r2", "desk-2", bgruntest.Done("Top story: rain in Lusaka.", "Top story: rain in Lusaka.", "search"))
	fin, _ = follower(f, nil).Follow(context.Background(), "r2", 0, true)
	if fin.Message() != "Top story: rain in Lusaka." {
		t.Fatalf("search answer: %q", fin.Message())
	}
}

// A reply that opens with "Done" itself never reads "Done ✓ Done. …".
func TestDoneNotSaidTwice(t *testing.T) {
	for in, want := range map[string]string{
		"Done. The file a.txt has been created with the content \"ready\".": "Done ✓ The file a.txt has been created with the content \"ready\".",
		"Done — `b.txt` created with \"ready\".":                            "Done ✓ `b.txt` created with \"ready\".",
		"Created c.txt with \"ready\".":                                     "Done ✓ Created c.txt with \"ready\".",
		"Done":                                                              "Done ✓",
		"done.txt now says ready.":                                          "Done ✓ done.txt now says ready.",
		"argus-43-ok":                                                       "Done ✓ argus-43-ok",
		"Done — out-1.txt contains `zephyr-1596`.":                          "Done ✓ out-1.txt contains `zephyr-1596`.",
	} {
		f := bgrun.Final{Kind: bgrun.KindDone, Action: true, Intent: "file", DeviceRan: true, DeviceOK: true, Reply: in}
		if got := f.Message(); got != want {
			t.Errorf("%q -> %q, want %q", in, got, want)
		}
	}
}

// The model's "<<follow-ups: …>>" line never reaches a receipt summary or a notification, and a
// reply that is only follow-ups still reads "Done ✓".
func TestFollowupsNeverInSummaryOrNotification(t *testing.T) {
	fu := "<<follow-ups: Open the file | Add another line | Delete it>>"
	cases := []struct {
		reply, sum string
	}{
		{"Created notes.txt with \"hello\".\n\n" + fu, ""},
		{"Created notes.txt with \"hello\".\n\n" + fu, "Created notes.txt. " + fu},
		{fu, ""},
		{fu, fu},
		{"\n" + fu + "\n", ""},
		{"Created notes.txt.\n<<follow-ups: Open it | Add a line", ""}, // unclosed
	}
	for _, c := range cases {
		var rc *bgrun.Receipt
		if c.sum != "" {
			rc = &bgrun.Receipt{Intent: "write", Summary: c.sum}
		}
		f := bgrun.Final{Kind: bgrun.KindDone, Action: true, Intent: "write", DeviceRan: true, DeviceOK: true, Reply: c.reply, Receipt: rc}
		msg, note := f.Message(), f.Notification()
		if strings.Contains(strings.ToLower(note), "follow") || strings.Contains(note, "<<") {
			t.Errorf("notification has follow-ups: %q", note)
		}
		if !strings.HasPrefix(msg, "Done ✓") {
			t.Errorf("reply %q: want Done ✓, got %q", c.reply, msg)
		}
		first := strings.SplitN(msg, "\n", 2)[0]
		if strings.Contains(first, "<<") {
			t.Errorf("Done line has follow-ups: %q", msg)
		}
		if strings.Count(msg, "<<") > 1 {
			t.Errorf("follow-ups doubled: %q", msg)
		}
		if body := bgrun.StripFollowups(c.reply); body == "" && !strings.HasPrefix(note, "Done") {
			t.Errorf("follow-ups-only reply: notification %q", note)
		}
	}
	// Through the follower too (Cloud run with a receipt).
	f := &bgruntest.Fake{}
	f.Seed("r1", "desk-1", bgruntest.Done(fu, fu, "write"))
	fin, _ := follower(f, nil).Follow(context.Background(), "r1", 0, true)
	if strings.Contains(fin.Notification(), "<<") {
		t.Fatalf("follower notification: %q", fin.Notification())
	}
}

// A reply that wears "Done ✓" itself or claims a save while no tool ran is never shown as done.
func TestNoToolClaimNeverDone(t *testing.T) {
	for _, r := range []string{"Done ✓ Got it - your gym locker number is 417.", "Saved! Your locker is 417.", "I've saved that your locker is 417."} {
		f := bgrun.Final{Kind: bgrun.KindDone, Action: true, Intent: "write", LocalIntent: true, Reply: r}
		m := f.Message()
		if strings.HasPrefix(m, "Done ✓") || (!strings.HasPrefix(r, "Done ✓") && !strings.Contains(m, bgrun.NoToolLine)) {
			t.Errorf("%q -> %q", r, m)
		}
	}
	// a question stays as it is
	q := "What time should I set it for?"
	if m := (bgrun.Final{Kind: bgrun.KindDone, Action: true, Intent: "reminder", Reply: q}).Message(); m != q {
		t.Errorf("question changed: %q", m)
	}
}

// Tools ran fine but the reply says the work didn't happen: never "Done ✓".
func TestFailedReplyNeverDone(t *testing.T) {
	for _, r := range []string{"Done ✓ I couldn't make that tool call. Please try again.", "I was unable to create the report.", "Sorry, that didn't work: I failed to write out.txt."} {
		f := bgrun.Final{Kind: bgrun.KindDone, Action: true, Intent: "write", LocalIntent: true, DeviceRan: true, DeviceOK: true, Reply: r}
		if f.Confirmed() || strings.HasPrefix(f.Message(), "Done ✓") {
			t.Errorf("%q -> %q", r, f.Message())
		}
	}
	ok := bgrun.Final{Kind: bgrun.KindDone, Action: true, Intent: "write", LocalIntent: true, DeviceRan: true, DeviceOK: true, Reply: "Created out.txt with zephyr-1."}
	if !ok.Confirmed() || !strings.HasPrefix(ok.Message(), "Done ✓") {
		t.Errorf("real done lost its mark: %q", ok.Message())
	}
}
