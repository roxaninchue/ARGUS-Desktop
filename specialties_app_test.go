package main

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/lacksonkapapahue/blaze-desktop/internal/specialties"
)

func TestSpecialtiesBindings(t *testing.T) {
	const sid = "sp_00112233445566778899"
	rules := []string{"Always aisle seats", "Never red-eyes"}
	gone := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case "GET /v1/specialties":
			if gone {
				w.Write([]byte(`{"data":[],"general_rules":[]}`))
				return
			}
			w.Write([]byte(`{"data":[{"id":"` + sid + `","name":"Booking","kind":"booking","steps":2,"rules":` + strconv.Itoa(len(rules)) + `}],"general_rules":["Reply in British English"]}`))
		case "GET /v1/specialties/" + sid:
			w.Write([]byte(`{"id":"` + sid + `","name":"Booking","steps":["Find options."],"rules":["` + strings.Join(rules, `","`) + `"]}`))
		case "DELETE /v1/specialties/" + sid + "/rules/0":
			rules = rules[1:]
			w.Write([]byte(`{}`))
		case "DELETE /v1/specialties/" + sid:
			gone = true
			w.Write([]byte(`{}`))
		default:
			w.WriteHeader(404)
			w.Write([]byte(`{"error":{"code":"not_found"}}`))
		}
	}))
	defer srv.Close()
	specialtiesAPIOverride = func(*App) *specialties.Client { return &specialties.Client{Base: srv.URL, Token: "bz_t"} }
	defer func() { specialtiesAPIOverride = nil }()
	a := &App{}

	v := a.SpecialtiesList()
	if v.Error != "" || len(v.Items) != 1 || v.Items[0].Rules != 2 || len(v.GeneralRules) != 1 {
		t.Fatalf("list: %+v", v)
	}
	v = a.SpecialtyGet(sid)
	if v.Open == nil || len(v.Open.Rules) != 2 {
		t.Fatalf("get: %+v", v)
	}
	v = a.SpecialtyRuleDelete(sid, 0)
	if v.Error != "" || v.Open == nil || len(v.Open.Rules) != 1 || v.Open.Rules[0] != "Never red-eyes" {
		t.Fatalf("rule delete: %+v", v)
	}
	v = a.SpecialtyDelete(sid)
	if v.Error != "" || len(v.Items) != 0 {
		t.Fatalf("delete: %+v", v)
	}
	// Deleting again (another device got there first) is not an error: the list is just fresh.
	if v = a.SpecialtyDelete(sid); v.Error != "" {
		t.Fatalf("second delete: %+v", v)
	}
	if v = a.SpecialtyGet("../admin"); v.Error == "" || v.Open != nil {
		t.Fatalf("bad id: %+v", v)
	}
}
