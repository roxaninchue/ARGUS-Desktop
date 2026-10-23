package specialties

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const sid = "sp_0123456789abcdef0123"

func fake(t *testing.T, calls *[]string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*calls = append(*calls, r.Method+" "+r.URL.Path)
		if r.Header.Get("Authorization") != "Bearer bz_test" {
			w.WriteHeader(401)
			w.Write([]byte(`{"error":{"code":"unauthorized"}}`))
			return
		}
		switch r.Method + " " + r.URL.Path {
		case "GET /v1/specialties":
			w.Write([]byte(`{"object":"list","data":[{"id":"` + sid + `","name":"Booking","kind":"booking","version":3,"uses":5,"steps":4,"rules":2,"updated_at":"2026-10-23T10:00:00.000Z"}],"general_rules":["Reply in British English"]}`))
		case "GET /v1/specialties/" + sid:
			w.Write([]byte(`{"object":"specialty","id":"` + sid + `","name":"Booking","kind":"booking","trigger":"book","steps":["Find options.","Confirm."],"rules":["Always aisle seats","Never red-eyes"],"version":3,"uses":5}`))
		case "GET /v1/specialties/general":
			w.Write([]byte(`{"id":"general","name":"General rules","rules":["Reply in British English"]}`))
		case "DELETE /v1/specialties/" + sid, "DELETE /v1/specialties/general":
			w.Write([]byte(`{"deleted":true}`))
		case "DELETE /v1/specialties/" + sid + "/rules/1":
			w.Write([]byte(`{"deleted_rule":1,"version":4}`))
		default:
			w.WriteHeader(404)
			w.Write([]byte(`{"error":{"message":"No such rule.","code":"not_found"}}`))
		}
	}))
}

func TestListGetDelete(t *testing.T) {
	var calls []string
	s := fake(t, &calls)
	defer s.Close()
	c := &Client{Base: s.URL + "/", Token: "bz_test"}
	ctx := context.Background()
	l, err := c.List(ctx)
	if err != nil || len(l.Data) != 1 || l.Data[0].Name != "Booking" || l.Data[0].Rules != 2 || len(l.GeneralRules) != 1 {
		t.Fatalf("list: %+v %v", l, err)
	}
	sp, err := c.Get(ctx, sid)
	if err != nil || len(sp.Steps) != 2 || sp.Rules[1] != "Never red-eyes" {
		t.Fatalf("get: %+v %v", sp, err)
	}
	g, err := c.Get(ctx, General)
	if err != nil || len(g.Rules) != 1 || g.Steps == nil {
		t.Fatalf("general: %+v %v", g, err)
	}
	if err := c.DeleteRule(ctx, sid, 1); err != nil {
		t.Fatal(err)
	}
	if err := c.Delete(ctx, sid); err != nil {
		t.Fatal(err)
	}
	var he *HTTPError
	if err := c.DeleteRule(ctx, sid, 7); !errors.As(err, &he) || he.Status != 404 || he.Code != "not_found" {
		t.Fatalf("missing rule: %v", err)
	}
	want := "GET /v1/specialties|GET /v1/specialties/" + sid + "|GET /v1/specialties/general|DELETE /v1/specialties/" + sid + "/rules/1|DELETE /v1/specialties/" + sid + "|DELETE /v1/specialties/" + sid + "/rules/7"
	if got := strings.Join(calls, "|"); got != want {
		t.Fatalf("calls\n got %s\nwant %s", got, want)
	}
}

func TestNeverSendsBadIDsOrWithoutSession(t *testing.T) {
	var calls []string
	s := fake(t, &calls)
	defer s.Close()
	ctx := context.Background()
	c := &Client{Base: s.URL, Token: "bz_test"}
	for _, id := range []string{"", "../admin", "sp_xyz", "sp_0123456789abcdef0123/rules", "General"} {
		if _, err := c.Get(ctx, id); err != ErrBadID {
			t.Fatalf("get %q: %v", id, err)
		}
		if err := c.Delete(ctx, id); err != ErrBadID {
			t.Fatalf("delete %q: %v", id, err)
		}
	}
	if err := c.DeleteRule(ctx, sid, -1); err == nil {
		t.Fatal("negative rule index sent")
	}
	out := &Client{Base: s.URL}
	if _, err := out.List(ctx); err != ErrSignedOut {
		t.Fatalf("signed out: %v", err)
	}
	if len(calls) != 0 {
		t.Fatalf("unexpected calls: %v", calls)
	}
	bad := &Client{Base: s.URL, Token: "bz_nope"}
	var he *HTTPError
	if _, err := bad.List(ctx); !errors.As(err, &he) || he.Status != 401 {
		t.Fatalf("401: %v", err)
	}
}
