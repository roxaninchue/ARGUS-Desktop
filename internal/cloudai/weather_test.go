package cloudai

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWeather(t *testing.T) {
	var q, auth string
	fail := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q, auth = r.URL.Path+"?"+r.URL.RawQuery, r.Header.Get("Authorization")
		if fail {
			w.WriteHeader(503)
			w.Write([]byte(`{"error":"weather_unavailable"}`))
			return
		}
		w.Write([]byte(`{"location":{"lat":-15.42,"lon":28.28},"units":"metric","temperature":31.1,"feels_like":29,"temperature_unit":"°C","condition":"Clear night","is_day":false,"humidity_pct":21,"wind_speed":8.4,"wind_unit":"km/h","observed_at":"2026-10-20T16:45:00.000Z","source":"Open-Meteo","summary":"31.1°C, clear night (Open-Meteo, 16:45 UTC)"}`))
	}))
	defer srv.Close()
	g := &Gateway{Base: srv.URL + "/v1", Token: func() string { return "tok" }, HC: srv.Client()}
	w, err := g.Weather(context.Background(), -15.4167, 28.2833, "")
	if err != nil || w.Source != "Open-Meteo" || w.Condition != "Clear night" || w.IsDay || auth != "Bearer tok" {
		t.Fatalf("%v %+v %q", err, w, auth)
	}
	if q != "/v1/weather?lat=-15.42&lon=28.28" {
		t.Fatalf("query %q", q)
	}
	fail = true
	if _, err := g.Weather(context.Background(), 1, 2, "imperial"); !errors.Is(err, ErrWeatherUnavailable) {
		t.Fatalf("503 must be weather_unavailable, got %v", err)
	}
	if q != "/v1/weather?lat=1.00&lon=2.00&units=imperial" {
		t.Fatalf("units query %q", q)
	}
	g.Token = func() string { return "" }
	if _, err := g.Weather(context.Background(), 1, 2, ""); !errors.Is(err, ErrSignedOut) {
		t.Fatalf("signed out: %v", err)
	}
}
