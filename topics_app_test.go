package main

import (
	"context"
	"strings"
	"testing"

	"github.com/lacksonkapapahue/blaze-desktop/internal/llm"
	"github.com/lacksonkapapahue/blaze-desktop/internal/longchat"
)

// The user's scenario through the app's real context path, one turn at a time as the
// chat would run it: hi → pizza → rocket → back to the pizza.
func TestTopicContextScenario(t *testing.T) {
	a := testApp(t)
	t.Cleanup(longchat.CloseDefault)
	a.prefs.Learn.Off = true // no background memory passes in the test
	judged := 0
	old := llmQuick
	llmQuick = func(ctx context.Context, a *App, msgs []llm.Message) (string, error) {
		judged++
		return `{"topic":"new","title":"Model rocket"}`, nil // only the rocket opener is ambiguous
	}
	t.Cleanup(func() { llmQuick = old })

	s := &session{id: "main"}
	a.cur = s
	turns := [][2]string{
		{"hi", "Hi! What can I do for you today?"},
		{"I want to make a pizza at home tonight. What dough recipe should I use?", "Neapolitan pizza dough: 500g flour, 325g water, 10g salt, 3g yeast."},
		{"Should I bake the pizza on a stone or a steel?", "A pizza steel gives a crisper base."},
		{"Now can you help me build a model rocket for the science fair?", "Use a cardboard body tube, balsa fins and an Estes C6-5 motor."},
		{"Important: the rocket has to be finished by Friday 24 October for the science fair deadline.", "Noted: paint Wednesday, test-fly Thursday."},
		{"What fin shape makes the rocket most stable?", "Clipped-delta fins are the most forgiving."},
	}
	for _, tr := range turns {
		s.hist = append(s.hist, llm.Message{Role: "user", Content: tr[0]})
		a.longContext(context.Background(), s, s.hist)
		s.hist = append(s.hist, llm.Message{Role: "assistant", Content: tr[1]})
	}
	s.hist = append(s.hist, llm.Message{Role: "user", Content: "Ok back to the pizza: how long should the dough rest in the fridge?"})
	out, extra := a.longContext(context.Background(), s, s.hist)

	if judged != 1 {
		t.Errorf("one small model call (the rocket opener), got %d", judged)
	}
	all := ""
	for _, m := range out {
		all += m.Content + "\n"
	}
	if !strings.Contains(all, "Neapolitan") || !strings.Contains(all, "rest in the fridge") {
		t.Errorf("pizza turns missing:\n%s", all)
	}
	for _, bad := range []string{"balsa", "Estes", "Clipped-delta", "What can I do for you"} {
		if strings.Contains(all, bad) || strings.Contains(extra, bad) {
			t.Errorf("leaked into the pizza context: %q", bad)
		}
	}
	if !strings.Contains(extra, "Friday 24 October") || !strings.Contains(extra, "ACTIVE TOPIC: Pizza") || !strings.Contains(extra, "Model rocket") {
		t.Errorf("topic text:\n%s", extra)
	}
	// Never forget: the rocket turns are still a search away.
	d, _ := longchat.Default()
	if hits := d.Search("main", "balsa fins", 5); len(hits) == 0 {
		t.Error("rocket turns must stay in the search index")
	}
	v := a.ChatTopics()
	if v.Current != "Pizza" || len(v.Topics) != 2 || !v.Topics[0].Active {
		t.Errorf("chip view: %+v", v)
	}
	// Switched off: the v0.18 path (every turn of this short chat) comes back.
	a.SetTopicMemory(false)
	out, extra = a.longContext(context.Background(), s, s.hist)
	if strings.Contains(extra, "ACTIVE TOPIC") || len(out) != len(s.hist) {
		t.Errorf("off: want the whole short chat, got %d of %d", len(out), len(s.hist))
	}
	if a.ChatTopics().Current != "" {
		t.Error("off: no chip")
	}
}
