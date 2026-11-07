package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lacksonkapapahue/argus-desktop/internal/selfknow"
	"github.com/lacksonkapapahue/argus-desktop/internal/tools"
	"rsc.io/qr"
)

// The whole Argus Link handshake as Argus Android's DesktopLink.kt does it: parse the
// blaze://pair link, pin the certificate, trade the one-time code for a token, ping,
// long-poll the inbox for phone_ask, reply, unpair.
func TestPairFlow(t *testing.T) {
	t.Setenv("ARGUS_HOME", t.TempDir())
	a := NewApp()
	a.pair = newPairHub(a)
	info := a.StartPairing()
	if info.Error != "" {
		t.Skip(info.Error)
	}
	u, _ := url.Parse(info.Link)
	q := u.Query()
	fp := q.Get("fp")
	// DesktopLink.pair: scheme blaze (Android parses it; kept), host pair, h (comma list), p, fp (64 hex), c, n.
	if u.Scheme != "blaze" || u.Host != "pair" || len(fp) != 64 || q.Get("c") == "" || q.Get("h") == "" || q.Get("p") != "7377" || q.Get("n") == "" {
		t.Fatalf("pairing link doesn't match DesktopLink.kt: %s", info.Link)
	}
	if !strings.HasPrefix(info.QR, "data:image/png;base64,") {
		t.Fatal("no QR image")
	}
	cl := &http.Client{Timeout: 40 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true,
		VerifyConnection: func(cs tls.ConnectionState) error {
			s := sha256.Sum256(cs.PeerCertificates[0].Raw)
			if hex.EncodeToString(s[:]) != fp {
				t.Fatal("fingerprint mismatch")
			}
			return nil
		}}}}
	base := "https://127.0.0.1:7377"
	// wrong code
	r, _ := cl.Post(base+"/v1/pair", "application/json", strings.NewReader(`{"code":"nope","name":"x"}`))
	if r.StatusCode != 403 {
		t.Fatal("wrong code accepted", r.StatusCode)
	}
	body, _ := json.Marshal(map[string]string{"code": q.Get("c"), "name": "Galaxy A12"})
	r, err := cl.Post(base+"/v1/pair", "application/json", bytes.NewReader(body))
	if err != nil || r.StatusCode != 200 {
		t.Fatal(err, r)
	}
	var pr map[string]string
	json.NewDecoder(r.Body).Decode(&pr)
	tok := pr["token"]
	// code is single-use
	r, _ = cl.Post(base+"/v1/pair", "application/json", bytes.NewReader(body))
	if r.StatusCode != 403 {
		t.Fatal("code reused")
	}
	get := func(path string) *http.Response {
		req, _ := http.NewRequest("GET", base+path, nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		r, err := cl.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	if r := get("/v1/ping"); r.StatusCode != 200 {
		t.Fatal("ping")
	} else {
		var pg map[string]string
		json.NewDecoder(r.Body).Decode(&pg)
		if pg["pc_name"] == "" || pg["version"] == "" {
			t.Fatalf("ping fields for DesktopLink.ping: %v", pg)
		}
	}
	// devices.json is what about_argus reads.
	if b, err := os.ReadFile(filepath.Join(os.Getenv("ARGUS_HOME"), "pair", "devices.json")); err != nil || !strings.Contains(string(b), "Galaxy A12") || strings.Contains(string(b), tok) {
		t.Fatalf("devices.json: %s %v", b, err)
	}
	// phone polls inbox; PC asks
	if s := a.pair.askPhone("x"); !strings.Contains(s, "isn't reachable") {
		t.Fatal("should be unreachable before any poll:", s)
	}
	get("/v1/inbox?wait=1") // registers the poll
	done := make(chan string)
	go func() {
		time.Sleep(300 * time.Millisecond)
		done <- a.pair.askPhone("what's my battery?")
	}()
	r = get("/v1/inbox?wait=5")
	var ib struct{ Requests []phoneReq }
	json.NewDecoder(r.Body).Decode(&ib)
	if len(ib.Requests) != 1 {
		t.Fatal("inbox", ib)
	}
	req, _ := http.NewRequest("POST", base+"/v1/inbox/"+ib.Requests[0].ID+"/reply", strings.NewReader(`{"text":"81%"}`))
	req.Header.Set("Authorization", "Bearer "+tok)
	cl.Do(req)
	if ans := <-done; ans != "The phone answered: 81%" {
		t.Fatal(ans)
	}
	if a.pair.connectedPhone() != "Galaxy A12" {
		t.Fatal("a phone that just polled counts as online")
	}
	a.Unpair(a.PairStatus().Devices[0].ID)
	if get("/v1/ping").StatusCode != 401 {
		t.Fatal("unpaired token still works")
	}
	a.pair.close()
}

// A phone task's approval is asked on the phone: the confirm shows up in the task's
// long-poll and the phone's answer (POST /v1/confirm) unblocks the tool.
func TestPhoneTaskConfirm(t *testing.T) {
	t.Setenv("ARGUS_HOME", t.TempDir())
	a := NewApp()
	a.pair = newPairHub(a)
	tk := &phoneTask{id: "t1", device: "d1", status: "running", changed: make(chan struct{}), created: time.Now()}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	env := a.pair.taskEnv(tk, ctx)
	got := make(chan bool)
	go func() { got <- env.Confirm("Delete build/", "rm -rf build") }()
	var c *taskConfirm
	for i := 0; i < 100 && c == nil; i++ {
		time.Sleep(10 * time.Millisecond)
		tk.mu.Lock()
		c = tk.confirm
		tk.mu.Unlock()
	}
	if c == nil || c.Title != "Delete build/" {
		t.Fatal("confirm not posted to the task")
	}
	c.answer <- true
	if !<-got {
		t.Fatal("phone's yes didn't reach the tool")
	}
	env.Status("Running tests")
	if len(tk.steps) != 1 {
		t.Fatal("status lines go to the phone")
	}
}

// about_argus gives real pairing steps and the live paired-phone list now that the PC side is in.
func TestAboutArgusLinkInBuild(t *testing.T) {
	a := selfTestApp(t)
	home := os.Getenv("ARGUS_HOME")
	_ = os.MkdirAll(filepath.Join(home, "pair"), 0o700)
	_ = os.WriteFile(filepath.Join(home, "pair", "devices.json"), []byte(`[{"id":"d1","name":"Galaxy A12","token_hash":"x","paired_at":"2026-10-12T10:00:00Z"}]`), 0o600)
	a.pair = newPairHub(a)
	defer a.pair.close()
	a.pair.mu.Lock()
	a.pair.lastPoll["d1"] = time.Now()
	a.pair.mu.Unlock()
	old := theApp
	theApp = a
	defer func() { theApp = old }()
	link := tools.Run(a.env, selfknow.ToolName, `{"section":"link"}`)
	for _, want := range []string{"Pair, step by step", "Galaxy A12", "Online now: Galaxy A12"} {
		if !strings.Contains(link, want) {
			t.Errorf("link section misses %q:\n%s", want, link)
		}
	}
	if strings.Contains(link, "NOT in this") {
		t.Error("still says the PC side is missing")
	}
	if sys := a.system("text mum"); !strings.Contains(sys, "PAIRED PHONE: Galaxy A12") {
		t.Error("system prompt should name the paired phone")
	}
}

func TestPhoneAskWithoutLink(t *testing.T) {
	dir := t.TempDir()
	env := &tools.Env{Root: dir, Shell: tools.NewShell("sh", "/bin/sh", dir)}
	if out := tools.Run(env, "phone_ask", `{"task":"Text Mum"}`); !strings.Contains(out, "No phone is paired") {
		t.Fatal(out)
	}
}

// The QR code fills its image (with a quiet zone), not just the top-left corner.
func TestQRImageScales(t *testing.T) {
	c, err := qr.Encode("blaze://pair?v=1&c=x", qr.M)
	if err != nil {
		t.Fatal(err)
	}
	img := qrImage(c, 8)
	d := (c.Size + 8) * 8
	if img.Bounds().Dx() != d {
		t.Fatal("size")
	}
	// The finder pattern's top-left module is black at (4,4) modules = (32,32) px; the quiet zone is white.
	dark := func(x, y int) bool { r, _, _, _ := img.At(x, y).RGBA(); return r < 0x8000 }
	if !dark(32+3, 32+3) || dark(3, 3) || !dark(d-32-4, 32+3) {
		t.Fatal("QR not drawn at scale")
	}
}

// Messages sent through the phone always need the user's own yes on the PC first, even
// when everything else is auto-approved.
func TestPhoneAskSendNeedsApproval(t *testing.T) {
	dir := t.TempDir()
	sent := ""
	asked := 0
	env := &tools.Env{Root: dir, Yolo: true, Shell: tools.NewShell("sh", "/bin/sh", dir),
		Phone:   func(task string) string { sent = task; return "The phone answered: sent" },
		Confirm: func(string, string) bool { return true },
		Approve: func(title, detail string) bool { asked++; return false }}
	if out := tools.Run(env, "phone_ask", `{"task":"Send WhatsApp to HomeGlow customer Ruth: your order shipped today"}`); !strings.Contains(out, "didn't approve") || sent != "" || asked != 1 {
		t.Fatalf("declined send went through: %q sent=%q", out, sent)
	}
	env.Approve = func(string, string) bool { asked++; return true }
	if out := tools.Run(env, "phone_ask", `{"task":"Send WhatsApp to Ruth: your order shipped today"}`); sent == "" || !strings.Contains(out, "sent") {
		t.Fatal("approved send should reach the phone")
	}
	sent = ""
	if tools.Run(env, "phone_ask", `{"task":"What's my battery level?"}`); sent == "" || asked != 2 {
		t.Fatal("a read-only phone request shouldn't ask")
	}
}
