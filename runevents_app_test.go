package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/lacksonkapapahue/argus-desktop/internal/bgrun"
	"github.com/lacksonkapapahue/argus-desktop/internal/bgrun/bgruntest"
	"github.com/lacksonkapapahue/argus-desktop/internal/privacy"
	"github.com/lacksonkapapahue/argus-desktop/internal/runevents"
)

// 0.19.16: offline the message stays "Sending" and the create is retried with the SAME
// client_msg_id; one run, one ack, one final. Then Sent (202), Read (first run event), a working
// step from the device tool's plain words, and no notification for any step.
type offlineAPI struct {
	*bgruntest.Fake
	fails int
	ids   []string
}

func (o *offlineAPI) Create(ctx context.Context, req bgrun.CreateReq) (*bgrun.Created, error) {
	o.ids = append(o.ids, req.ClientMsgID)
	if o.fails > 0 {
		o.fails--
		return nil, errors.New("dial tcp: lookup blaze-gateway: no such host")
	}
	return o.Fake.Create(ctx, req)
}

func TestDeskOfflineRetrySameMessageNoDuplicate(t *testing.T) {
	d := deskSetup(t, true, true)
	d.fake.AckFor = reminderAck
	d.fake.ScriptFor = func(bgrun.CreateReq) []bgrun.View {
		return []bgrun.View{bgruntest.Running(), bgruntest.NeedsDevice("c1", "schedule_add", `{"at":"9:00","text":"call mum"}`), bgruntest.Done("Reminder set.", "Reminder set.", "reminder")}
	}
	api := &offlineAPI{Fake: d.fake, fails: 2}
	runsAPIOverride = api
	old := runsCreateSleep
	runsCreateSleep = func(context.Context, time.Duration) error { return nil }
	t.Cleanup(func() { runsCreateSleep = old })

	if _, ok := d.turn(context.Background(), "remind me to call mum at 9 tomorrow"); !ok {
		t.Fatal("not handled as a run")
	}
	d.tap.waitFor(t, "argus_msg")
	bgRuns.Wait()
	if len(api.ids) != 3 || api.ids[0] != api.ids[1] || api.ids[1] != api.ids[2] {
		t.Fatalf("retries must reuse the client_msg_id: %v", api.ids)
	}
	if created, _, _ := d.fake.Snapshot(); len(created) != 1 {
		t.Fatalf("%d runs created, want 1", len(created))
	}
	if n := len(d.tap.find("argus_msg")); n != 1 {
		t.Fatalf("%d finals, want 1", n)
	}
	d.tap.mu.Lock()
	runs := append([]string(nil), d.tap.runs...)
	d.tap.mu.Unlock()
	var types []string
	var step string
	for _, r := range runs {
		var e runevents.Ev
		_ = json.Unmarshal([]byte(r), &e)
		types = append(types, e.Type)
		if e.Type == runevents.Step {
			step = e.Text
		}
		if privacy.Leaks(e.Text) || strings.ContainsAny(e.Text, "{}") || len([]rune(e.Text)) > runevents.Max {
			t.Fatalf("unsafe pill text: %s", r)
		}
	}
	got := strings.Join(types, ",")
	if !strings.HasPrefix(got, "message_sending,message_sending,message_sending,message_received,message_read") {
		t.Fatalf("run events: %s", got)
	}
	if !strings.Contains(runs[1], `"offline":true`) || strings.Contains(runs[0], `"offline":true`) {
		t.Fatalf("offline flag: %v", runs[:3])
	}
	if step == "" {
		t.Fatalf("no working step for the device tool: %v", runs)
	}
	if _, notes := d.tap.snap(); len(notes) != 0 {
		t.Fatalf("steps must never notify: %v", notes)
	}
}

// The chat chrome never carries a provider or model (the gateway's done event names them).
func TestEmitScrubsProviderAndModel(t *testing.T) {
	t.Setenv("ARGUS_DEV", "")
	for _, c := range []struct {
		ev string
		d  any
	}{
		{"done", map[string]any{"route": "Groq · llama-3.3-70b", "note": "Cerebras didn't answer, so Groq did", "provider": "groq", "model": "llama-3.3-70b"}},
		{"error", turnError(errors.New("sambanova 429: Meta-Llama-3.3-70B-Instruct rate limited"))},
		{"status", "Asking OpenRouter (deepseek-chat-v3)"},
		{"trace", &turnTrace{Turn: "t1", Steps: []traceStep{{N: 1, Route: "Groq", Provider: "groq", Model: "llama-3.3-70b", Note: "via Cerebras"}}}},
		{"route", map[string]any{"route": "Gemini 2.5 Flash", "note": "Gemini didn't answer, so Qwen did"}},
	} {
		b, _ := json.Marshal(privacy.Event(c.ev, c.d))
		if privacy.Leaks(string(b)) {
			t.Errorf("%s leaks %v: %s", c.ev, privacy.Found(string(b)), b)
		}
	}
}
