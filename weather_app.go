package main

// get_weather (v0.19.7, gateway 0.6.8): weather always comes from Blaze Cloud's /v1/weather, in chat
// and on the Home card, so the two agree and Blaze never answers weather from a web search or a
// guess. Place names are turned into coordinates first; with no place, the PC's approximate
// location is used.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lacksonkapapahue/blaze-desktop/internal/cloudai"
	"github.com/lacksonkapapahue/blaze-desktop/internal/home"
	"github.com/lacksonkapapahue/blaze-desktop/internal/llm"
)

const weatherLine = "\nWEATHER: for any weather question call get_weather and quote the source and time it returns. Never use web search, memory or a guess for weather. If it says weather is unavailable, say so.\n"

func weatherTool() llm.Tool {
	var t llm.Tool
	t.Type = "function"
	t.Function.Name = "get_weather"
	t.Function.Description = "Current weather for a place. ALWAYS use this for any weather question; never answer weather from web search or memory. Report temperature, condition, and the source + observed time it returns. Give lat/lon, or a place name, or nothing for the user's own location."
	t.Function.Parameters = map[string]any{"type": "object", "properties": map[string]any{
		"lat":   map[string]any{"type": "number"},
		"lon":   map[string]any{"type": "number"},
		"units": map[string]any{"type": "string", "enum": []string{"metric", "imperial"}},
		"place": map[string]any{"type": "string", "description": "A city or place name, when you don't have lat/lon."},
	}}
	return t
}

// runWeather executes get_weather.
func (a *App) runWeather(ctx context.Context, raw string) string {
	var p struct {
		Lat   *float64 `json:"lat"`
		Lon   *float64 `json:"lon"`
		Units string   `json:"units"`
		Place string   `json:"place"`
	}
	_ = json.Unmarshal([]byte(raw), &p)
	lat, lon, name, err := weatherPlace(ctx, p.Lat, p.Lon, p.Place)
	if err != nil {
		return "Error: " + err.Error()
	}
	w, err := currentWeather(ctx, lat, lon, p.Units)
	if err != nil {
		return weatherErr(err)
	}
	out := map[string]any{"place": name, "weather": w}
	b, _ := json.Marshal(out)
	return string(b)
}

func weatherErr(err error) string {
	switch {
	case errors.Is(err, cloudai.ErrSignedOut):
		return "Weather is unavailable: sign in to your Blaze account (Settings › Account) to get weather. Don't guess or search for it."
	default:
		return "Weather is unavailable right now (weather_unavailable). Tell the user; don't guess or search for it."
	}
}

func currentWeather(ctx context.Context, lat, lon float64, units string) (*cloudai.Weather, error) {
	if cloudGateway == nil {
		return nil, cloudai.ErrWeatherUnavailable
	}
	return cloudGateway.Weather(ctx, lat, lon, units)
}

// weatherPlace: lat/lon as given, else geocode the place, else the PC's approximate location.
func weatherPlace(ctx context.Context, lat, lon *float64, place string) (float64, float64, string, error) {
	if lat != nil && lon != nil {
		return *lat, *lon, strings.TrimSpace(place), nil
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if place = strings.TrimSpace(place); place != "" {
		_, body, err := homeNet(ctx, home.GeocodeURL(place, "en"), nil)
		if err != nil {
			return 0, 0, "", fmt.Errorf("couldn't look up %q", place)
		}
		p, ok := home.ParsePlace(body)
		if !ok {
			return 0, 0, "", fmt.Errorf("couldn't find a place called %q", place)
		}
		return p.Lat, p.Lon, p.Name, nil
	}
	p, err := homeLocate(ctx)
	if err != nil || (p.Lat == 0 && p.Lon == 0) {
		return 0, 0, "", fmt.Errorf("don't know where the user is; ask them for a city")
	}
	return p.Lat, p.Lon, p.Name, nil
}

// homeWeatherNow feeds the Home weather card the same Blaze Cloud numbers chat gets.
func homeWeatherNow(ctx context.Context, lat, lon float64, fahrenheit bool) (home.Current, error) {
	units := "metric"
	if fahrenheit {
		units = "imperial"
	}
	w, err := currentWeather(ctx, lat, lon, units)
	if err != nil {
		return home.Current{}, err
	}
	c := home.Current{Temp: w.Temperature, Unit: w.TempUnit, Condition: w.Condition, FeelsLike: w.FeelsLike, WindUnit: w.WindUnit, Source: w.Source}
	if w.WindSpeed != nil {
		c.Wind = *w.WindSpeed
	}
	if t, err := time.Parse(time.RFC3339, w.ObservedAt); err == nil {
		c.Observed = t
	}
	return c, nil
}
