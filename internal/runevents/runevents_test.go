package runevents

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/lacksonkapapahue/argus-desktop/internal/bgrun"
	"github.com/lacksonkapapahue/argus-desktop/internal/bgrun/bgruntest"
	"github.com/lacksonkapapahue/argus-desktop/internal/privacy"
)

func TestSanitize(t *testing.T) {
	cases := map[string]string{
		"Reading your CSV":                         "Reading your CSV",
		"Searching the web":                        "Searching the web",
		`run_shell {"command":"ls -la ~"}`:         "",
		`Reading {"path":"/home/x/a.csv"}`:         "Reading",
		"Asking Groq llama-3.3-70b":                "",
		"Handing this to Luke":                     "",
		"read_file(path=a.csv) Reading your notes": "Reading your notes",
		"": "",
	}
	for in, want := range cases {
		if got := Sanitize(in); got != want {
			t.Errorf("Sanitize(%q) = %q, want %q", in, got, want)
		}
	}
	long := Sanitize(strings.Repeat("Reading every single file in your project folder ", 4))
	if n := len([]rune(long)); n > Max || !strings.HasSuffix(long, "…") {
		t.Fatalf("not clipped to %d: %q (%d)", Max, long, n)
	}
}

func TestTrackerStubAndSpecialties(t *testing.T) {
	tr := &Tracker{RunID: "run_1", ClientMsgID: "c1"}
	var all []Ev
	all = append(all, tr.FromRun(bgrun.Event{Type: "running"})...)
	all = append(all, tr.FromToolCall("read_file", "Reading your CSV")...)
	all = append(all, tr.FromToolCall("read_file", "Reading your CSV")...) // same line: no new event
	all = append(all, tr.FromRun(bgrun.Event{Type: "specialty_loaded", Text: "loaded data-analyst (groq)"})...)
	all = append(all, tr.FromRun(bgrun.Event{Type: "step", Kind: "specialty", Text: "Using a specialty"})...) // sent as both: shown once
	all = append(all, tr.FromRun(bgrun.Event{Type: "specialty_updated", Text: "updated data-analyst"})...)
	all = append(all, tr.FromRun(bgrun.Event{Type: "step", Text: "Searching the web"})...)
	all = append(all, tr.FromRun(bgrun.Event{Type: "step", Text: `web_search {"q":"x"}`})...)
	var got []string
	for _, e := range all {
		got = append(got, e.Type+":"+e.Text)
		if privacy.Leaks(e.Text) || strings.Contains(e.Text, "analyst") || strings.Contains(e.Text, "{") {
			t.Fatalf("leak in %+v", e)
		}
	}
	want := "message_read:|step:Reading your CSV|specialty_loaded:Bringing in a specialist|specialty_updated:Updating a specialist|step:Searching the web"
	if strings.Join(got, "|") != want {
		t.Fatalf("got %s\nwant %s", strings.Join(got, "|"), want)
	}
	if ev := (&Tracker{}).FromToolCall("dispatch_specialist", "Briefing data-analyst"); len(ev) != 2 || ev[1].Text != SpecialtyLoadedText {
		t.Fatalf("dispatch_specialist with nothing usable: last resort, never the name: %+v", ev)
	}
	if ev := (&Tracker{Request: "Fix the failing build please"}).FromToolCall("dispatch_specialist", "Briefing data-analyst"); len(ev) != 2 || ev[1].Text != "Getting help to fix the failing build please" {
		t.Fatalf("dispatch_specialist falls back to the request: %+v", ev)
	}
	if ev := (&Tracker{}).FromToolCall("dispatch_specialist", "Getting help to chart revenue by store"); len(ev) != 2 || ev[1].Text != "Getting help to chart revenue by store" {
		t.Fatalf("friendlyTool's help line is kept: %+v", ev)
	}
}

// Specialty pills say what the help is for from the event's own detail, vary with it, never name
// the specialty/model/provider, and use the general line only when nothing usable is there.
func TestHelpLineFromDetail(t *testing.T) {
	static := map[string]bool{SpecialtyLoadedText: true, SpecialtyUpdatedText: true, "Using a specialty": true, "Updating a specialty": true}
	tr := &Tracker{RunID: "r", Request: "Book me a table for two at 8"}
	cases := []struct {
		ev   bgrun.Event
		want string
	}{
		{bgrun.Event{Type: "specialty_loaded", Text: "Using a specialty", Detail: "fix the build error", Name: "build-fixer"}, "Getting help to fix the build error"},
		{bgrun.Event{Type: "specialty_loaded", Text: "Using a specialty", Task: "Charting revenue by store"}, "Getting help with charting revenue by store"},
		{bgrun.Event{Type: "step", Kind: "specialty", Text: "Using a specialty", Detail: "plan the Lusaka trip with data-analyst (groq llama)", OwnerName: "data-analyst"}, "Getting help to plan the Lusaka trip"},
		{bgrun.Event{Type: "specialty_updated", Text: "Updating a specialty", Detail: "Always book aisle seats", Name: "flights"}, "Saving what I learned: always book aisle seats"},
		{bgrun.Event{Type: "specialty_loaded", Text: "Using a specialty"}, "Getting help to book me a table for two at 8"},
	}
	seen := map[string]bool{}
	for _, c := range cases {
		ev := tr.FromRun(c.ev)
		var got string
		for _, e := range ev {
			if e.Text != "" {
				got = e.Text
			}
		}
		if got != c.want {
			t.Errorf("%+v: got %q want %q", c.ev, got, c.want)
		}
		if static[got] || privacy.Leaks(got) || strings.Contains(got, "analyst") || strings.Contains(got, "fixer") || len([]rune(got)) > Max {
			t.Errorf("static / leaky / long line %q", got)
		}
		seen[got] = true
	}
	if len(seen) != len(cases) {
		t.Errorf("lines do not vary with the detail: %v", seen)
	}
	if got := HelpLine(false, "loaded data-analyst (groq)", "", "data-analyst"); got != SpecialtyLoadedText {
		t.Errorf("nothing usable: last resort %q", got)
	}
	if got := HelpLine(true, "", ""); got != SpecialtyUpdatedText {
		t.Errorf("nothing usable (updated): last resort %q", got)
	}
	if got := HelpLine(false, "Chart sales from cafe-sales-sept.csv", "", "data-analyst"); got != "Getting help to chart sales from cafe-sales-sept.csv" {
		t.Errorf("file names stay: %q", got)
	}
	if got := Sanitize("Reading my_data.csv"); got != "Reading my_data.csv" {
		t.Errorf("snake_case file name dropped: %q", got)
	}
}

// Offline: the same client_msg_id on every try, one run at the end, Sending until it lands.
func TestCreateOfflineRetrySameID(t *testing.T) {
	f := &bgruntest.Fake{}
	fails := 3
	api := &flaky{Fake: f, fails: &fails}
	req := bgrun.CreateReq{ClientMsgID: bgrun.NewClientMsgID()}
	var evs []Ev
	var waits []time.Duration
	cr, err := Create(context.Background(), api, req, func(e Ev) { evs = append(evs, e) },
		func(_ context.Context, d time.Duration) error { waits = append(waits, d); return nil })
	if err != nil || cr == nil {
		t.Fatalf("create: %v", err)
	}
	if len(api.ids) != 4 {
		t.Fatalf("tries %d", len(api.ids))
	}
	for _, id := range api.ids {
		if id != req.ClientMsgID {
			t.Fatalf("retry changed client_msg_id: %v", api.ids)
		}
	}
	created, _, _ := f.Snapshot()
	if len(created) != 1 {
		t.Fatalf("runs created %d, want 1", len(created))
	}
	if len(evs) != 5 || evs[0].Type != Sending || evs[0].Offline || !evs[3].Offline || evs[3].Attempt != 4 || evs[4].Type != Received || evs[4].RunID != cr.RunID {
		t.Fatalf("events %+v", evs)
	}
	if waits[0] != time.Second || waits[2] != 4*time.Second || RetryWaits[4] != 16*time.Second || RetryWaits[5] != 30*time.Second {
		t.Fatalf("waits %v", waits)
	}
}

// A try that landed on the gateway but whose answer was lost: the retry gets the same run back.
func TestCreateLostAnswerNoDuplicate(t *testing.T) {
	f := &bgruntest.Fake{}
	api := &lostAnswer{Fake: f}
	req := bgrun.CreateReq{ClientMsgID: "c-lost-1"}
	cr, err := Create(context.Background(), api, req, nil, func(context.Context, time.Duration) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	created, _, _ := f.Snapshot()
	if len(f.Runs()) != 1 || cr.RunID != "run_fake1" || !cr.Existing {
		t.Fatalf("duplicate run: created=%d runs=%v cr=%+v", len(created), f.Runs(), cr)
	}
}

func TestCreateHTTPErrorNotRetried(t *testing.T) {
	f := &bgruntest.Fake{CreateErr: &bgrun.HTTPError{Code: 404, Body: "no runs"}}
	n := 0
	_, err := Create(context.Background(), f, bgrun.CreateReq{ClientMsgID: "c2"}, func(e Ev) { n++ }, func(context.Context, time.Duration) error { return nil })
	if bgrun.StatusOf(err) != 404 || n != 1 {
		t.Fatalf("err %v sends %d", err, n)
	}
}

type flaky struct {
	*bgruntest.Fake
	fails *int
	ids   []string
}

func (f *flaky) Create(ctx context.Context, req bgrun.CreateReq) (*bgrun.Created, error) {
	f.ids = append(f.ids, req.ClientMsgID)
	if *f.fails > 0 {
		*f.fails--
		return nil, errors.New("dial tcp: network is unreachable")
	}
	return f.Fake.Create(ctx, req)
}

type lostAnswer struct {
	*bgruntest.Fake
	n int
}

func (l *lostAnswer) Create(ctx context.Context, req bgrun.CreateReq) (*bgrun.Created, error) {
	l.n++
	cr, err := l.Fake.Create(ctx, req)
	if l.n == 1 {
		return nil, errors.New("read: connection reset by peer") // it landed; the answer didn't
	}
	return cr, err
}

// The Site contract (gateway 0.8.45): Sent from the 2xx / message_received, Read only from
// message_read (or read != null on a retry), steps of kind specialty shown once, generic.
func TestContractDelivery(t *testing.T) {
	cr := &bgrun.Created{RunID: "run_c", ClientMsgID: "m1", Received: &bgrun.Receipt0{MessageID: "msg_1"}}
	if ev := Delivered(cr, "m1"); len(ev) != 1 || ev[0].Type != Received || ev[0].RunID != "run_c" {
		t.Fatalf("202: %+v", ev)
	}
	tr := For(cr, "m1")
	if ev := tr.Ack(); len(ev) != 0 {
		t.Fatalf("the ack is a template, not Read: %+v", ev)
	}
	var types []string
	for _, e := range []bgrun.Event{{Type: "message_received"}, {Type: "status", Status: "queued"}, {Type: "ack"}, {Type: "message_read"},
		{Type: "status", Status: "running"}, {Type: "specialty_loaded", Text: "Using a specialty"}, {Type: "step", Kind: "specialty", Text: "Using a specialty"},
		{Type: "step", Kind: "step", Text: "Opening the booking page"}, {Type: "message_read"}} {
		for _, x := range tr.FromRun(e) {
			types = append(types, x.Type+":"+x.Text)
		}
	}
	want := "message_received:|message_read:|specialty_loaded:Bringing in a specialist|step:Opening the booking page"
	if strings.Join(types, "|") != want {
		t.Fatalf("got  %s\nwant %s", strings.Join(types, "|"), want)
	}
	// a retry the gateway already picked up: Sent and Read at once
	cr2 := &bgrun.Created{RunID: "run_c", Existing: true, Received: &bgrun.Receipt0{MessageID: "msg_1"}, Read: &bgrun.Receipt0{MessageID: "msg_1"}}
	if ev := Delivered(cr2, "m1"); len(ev) != 2 || ev[1].Type != Read {
		t.Fatalf("retry with read: %+v", ev)
	}
	if ev := For(cr2, "m1").FromRun(bgrun.Event{Type: "message_read"}); len(ev) != 0 {
		t.Fatalf("Read twice: %+v", ev)
	}
}

func TestRetry429ButNot400(t *testing.T) {
	for code, want := range map[int]int{429: 2, 408: 2, 400: 1, 401: 1} {
		f := &bgruntest.Fake{}
		api := &statusOnce{Fake: f, code: code}
		n := 0
		_, _ = Create(context.Background(), api, bgrun.CreateReq{ClientMsgID: "c"}, func(e Ev) {
			if e.Type == Sending {
				n++
			}
		}, func(context.Context, time.Duration) error { return nil })
		if n != want {
			t.Errorf("HTTP %d: %d tries, want %d", code, n, want)
		}
	}
}

type statusOnce struct {
	*bgruntest.Fake
	code int
	done bool
}

func (s *statusOnce) Create(ctx context.Context, req bgrun.CreateReq) (*bgrun.Created, error) {
	if !s.done {
		s.done = true
		return nil, &bgrun.HTTPError{Code: s.code}
	}
	return s.Fake.Create(ctx, req)
}
