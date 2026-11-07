package topics

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/lacksonkapapahue/argus-desktop/internal/llm"
)

func u(s string) llm.Message { return llm.Message{Role: "user", Content: s} }
func a(s string) llm.Message { return llm.Message{Role: "assistant", Content: s} }

// The user's own scenario: hi → pizza → rocket → back to the pizza.
func scenario() []llm.Message {
	return []llm.Message{
		u("hi"),
		a("Hi! What can I do for you today?"),
		// pizza (messages 2-7)
		u("I want to make a pizza at home tonight. What dough recipe should I use?"),
		a("For a Neapolitan-style pizza dough: 500g flour, 325g water, 10g salt, 3g yeast. Knead 10 minutes and let it rise."),
		u("Should I bake the pizza on a stone or a steel?"),
		a("A pizza steel heats faster and gives a crisper base; preheat it for 45 minutes at the highest oven setting."),
		u("What sauce goes on the pizza? I like it simple."),
		a("Crush San Marzano tomatoes with salt and a little olive oil. Don't cook the sauce; it cooks on the pizza."),
		// rocket (messages 8-13)
		u("Now can you help me build a model rocket for the science fair?"),
		a("Sure! Use a cardboard body tube, three balsa fins and an Estes C6-5 motor with a parachute recovery system."),
		u("Important: the rocket has to be finished by Friday 24 October for the science fair deadline."),
		a("Noted. With balsa fins glued tonight, the rocket can be painted Wednesday and test-flown Thursday."),
		u("What fin shape makes the rocket most stable?"),
		a("Clipped-delta fins are the most forgiving: put the centre of pressure behind the centre of gravity."),
		// back to pizza
		u("Ok back to the pizza: how long should the dough rest in the fridge?"),
	}
}

var rocketWords = []string{"balsa", "Estes", "Clipped-delta", "fin shape", "cardboard body tube", "test-flown"}

func run(t *testing.T, h []llm.Message, j Judge) (*State, []llm.Message, string) {
	t.Helper()
	s := New()
	s.Segment(h, SegOpts{Judge: j, JudgeFrom: len(h) - 1})
	msgs, extra := s.Assemble(h, Opts{Budget: 12000})
	return s, msgs, extra
}

func TestUserScenarioBackToPizza(t *testing.T) {
	h := scenario()
	s, msgs, extra := run(t, h, nil)

	if got := s.TurnTopic[0]; got != 0 {
		t.Fatalf("hi should attach to no topic, got %d", got)
	}
	pizza, rocket := s.TurnTopic[2], s.TurnTopic[8]
	if pizza == 0 || rocket == 0 || pizza == rocket {
		t.Fatalf("want two topics, got pizza=%d rocket=%d (%v)", pizza, rocket, s.TurnTopic)
	}
	for _, i := range []int{4, 6} {
		if s.TurnTopic[i] != pizza {
			t.Errorf("turn %d should stay on pizza, got %d", i, s.TurnTopic[i])
		}
	}
	for _, i := range []int{10, 12} {
		if s.TurnTopic[i] != rocket {
			t.Errorf("turn %d should stay on rocket, got %d", i, s.TurnTopic[i])
		}
	}
	if s.TurnTopic[14] != pizza || s.Active != pizza {
		t.Fatalf("back to the pizza should return to topic %d, got %d (active %d)", pizza, s.TurnTopic[14], s.Active)
	}
	if s.Get(pizza).Title != "Pizza" || s.Get(rocket).Title != "Model rocket" {
		t.Errorf("titles: %q %q", s.Get(pizza).Title, s.Get(rocket).Title)
	}

	// The prompt for the last message: pizza turns, no rocket turns, no "hi".
	all := ""
	for _, m := range msgs {
		all += m.Content + "\n"
	}
	for _, want := range []string{"Neapolitan", "pizza steel", "San Marzano", "rest in the fridge"} {
		if !strings.Contains(all, want) {
			t.Errorf("pizza context should contain %q", want)
		}
	}
	for _, bad := range rocketWords {
		if strings.Contains(all, bad) || strings.Contains(extra, bad) {
			t.Errorf("rocket turn leaked into the pizza context: %q", bad)
		}
	}
	if strings.Contains(all, "What can I do for you today") {
		t.Error("the opening small talk should not be sent with the pizza topic")
	}
	if len(msgs) != 7 || msgs[0].Content != h[2].Content {
		t.Errorf("want the 6 pizza messages + the current one, got %d", len(msgs))
	}
	// The rocket deadline was marked important: it carries over.
	if !strings.Contains(extra, "CARRIED OVER FROM") || !strings.Contains(extra, "Friday 24 October") {
		t.Errorf("rocket deadline should carry over into the pizza topic:\n%s", extra)
	}
	// The rocket topic is listed (one line) so the model knows it exists.
	if !strings.Contains(extra, "OTHER TOPICS") || !strings.Contains(extra, "Model rocket") {
		t.Errorf("other topics index missing:\n%s", extra)
	}
	if !strings.Contains(extra, "ACTIVE TOPIC: Pizza") {
		t.Errorf("active topic header missing:\n%s", extra)
	}
}

// Without the explicit cue, the pizza words alone bring the pizza back.
func TestReturnByKeywords(t *testing.T) {
	h := scenario()
	h[len(h)-1] = u("How long should the pizza dough rest in the fridge?")
	s, msgs, _ := run(t, h, nil)
	if s.Active != s.TurnTopic[2] {
		t.Fatalf("keyword return failed: active %d", s.Active)
	}
	for _, m := range msgs {
		for _, bad := range rocketWords {
			if strings.Contains(m.Content, bad) {
				t.Fatalf("rocket leaked: %q", bad)
			}
		}
	}
}

// While on the rocket, the pizza turns aren't sent either, and nothing carries over.
func TestRocketContextHasNoPizza(t *testing.T) {
	h := scenario()[:13] // up to "What fin shape…"
	s, msgs, extra := run(t, h, nil)
	if s.Active != s.TurnTopic[8] {
		t.Fatalf("active should be rocket")
	}
	for _, m := range msgs {
		if strings.Contains(m.Content, "Neapolitan") || strings.Contains(m.Content, "San Marzano") {
			t.Fatal("pizza turn sent with the rocket topic")
		}
	}
	if strings.Contains(extra, "CARRIED OVER FROM") {
		t.Errorf("nothing about the pizza was important, nothing should carry over: %+v", s.Carry)
	}
}

func TestSmallTalkAndFollowups(t *testing.T) {
	for _, s := range []string{"hi", "Hello!", "thanks!", "ok cool", "good morning", "bye", "lol", "thank you so much"} {
		if !IsSmallTalk(s) {
			t.Errorf("%q should be small talk", s)
		}
	}
	for _, s := range []string{"hi, how do I bake bread?", "ok make it bigger", "pizza", "thanks, now fix the build"} {
		if IsSmallTalk(s) {
			t.Errorf("%q isn't small talk", s)
		}
	}
	h := append(scenario()[:8], u("thanks!"), a("Enjoy the pizza!"), u("make it bigger"))
	s, msgs, _ := run(t, h, nil)
	if s.TurnTopic[8] != 0 {
		t.Error("thanks attaches to no topic")
	}
	if s.TurnTopic[10] != s.TurnTopic[2] {
		t.Error("an anaphoric follow-up stays on the active topic")
	}
	found := false // small talk inside the pizza flow is kept with it
	for _, m := range msgs {
		found = found || m.Content == "thanks!"
	}
	if !found {
		t.Error("small talk inside the active topic should be sent")
	}
}

func TestNewTopicCueAndJudge(t *testing.T) {
	h := append(scenario()[:8], u("Different question: what's the capital of Peru?"))
	s, _, _ := run(t, h, nil)
	if s.TurnTopic[8] == s.TurnTopic[2] || s.TurnTopic[8] == 0 {
		t.Fatal("'different question' should open a new topic")
	}
	// No overlap and no anaphora: the judge decides; here it says "same topic".
	h = append(scenario()[:8], u("Which toppings go best on a margherita?"))
	called := 0
	judge := func(text string, act *Topic, ts []*Topic) (int, string, error) {
		called++
		if !strings.Contains(JudgeInput(text, act, ts), "pizza") {
			t.Error("judge input should list the pizza topic")
		}
		return act.ID, "", nil
	}
	s, _, _ = run(t, h, judge)
	if called != 1 || s.TurnTopic[8] != s.TurnTopic[2] {
		t.Fatalf("judge should be called once and keep pizza (called %d, topic %d)", called, s.TurnTopic[8])
	}
	// The judge is never called for messages before JudgeFrom.
	called = 0
	s = New()
	s.Segment(scenario(), SegOpts{Judge: judge, JudgeFrom: 1 << 30})
	if called != 0 {
		t.Error("history must be re-read without model calls")
	}
	if id, title, err := ParseJudge("```json\n{\"topic\": \"new\", \"title\": \"Peru\"}\n```"); err != nil || id != 0 || title != "Peru" {
		t.Errorf("ParseJudge new: %d %q %v", id, title, err)
	}
	if id, _, err := ParseJudge(`{"topic": 3}`); err != nil || id != 3 {
		t.Errorf("ParseJudge id: %d %v", id, err)
	}
}

func TestIncrementalMatchesFullRead(t *testing.T) {
	h := scenario()
	inc := New()
	for i := 1; i <= len(h); i++ {
		inc.Segment(h[:i], SegOpts{})
	}
	full := New()
	full.Segment(h, SegOpts{})
	if fmt.Sprint(inc.TurnTopic) != fmt.Sprint(full.TurnTopic) || inc.Active != full.Active {
		t.Fatalf("incremental %v != full %v", inc.TurnTopic, full.TurnTopic)
	}
	// It survives a JSON round trip (that is how longchat stores it).
	b, _ := json.Marshal(inc)
	var back State
	if err := json.Unmarshal(b, &back); err != nil || back.Active != inc.Active || len(back.Topics) != 2 || back.TurnTopic[14] != inc.TurnTopic[14] {
		t.Fatalf("round trip: %v", err)
	}
	// A retry (the chat got shorter) truncates instead of starting over.
	back.Segment(h[:13], SegOpts{})
	if back.Assigned != 13 || back.Active != back.TurnTopic[8] || back.TurnTopic[14] != 0 {
		t.Fatalf("truncate: assigned %d active %d", back.Assigned, back.Active)
	}
}

func TestCarryExtraction(t *testing.T) {
	cases := map[string]string{
		"The report is due by Friday.":                  "deadline",
		"Let's go with the blue logo.":                  "decision",
		"I'm allergic to peanuts.":                      "preference",
		"Remember that the API key lives in the vault.": "important",
		"Use 500g flour and 325g water.":                "",
		"Can you make the sauce less salty?":            "",
		"Make sure the dough is cold.":                  "",
	}
	for in, want := range cases {
		got := ExtractCarry("user", in)
		k := ""
		if len(got) > 0 {
			k = got[0].Kind
		}
		if k != want {
			t.Errorf("%q: kind %q, want %q", in, k, want)
		}
	}
	if len(ExtractCarry("assistant", "The report is due by Friday.")) != 0 {
		t.Error("assistant lines only carry promises")
	}
	if c := ExtractCarry("assistant", "I'll remind you on Thursday to test-fly it."); len(c) != 1 || c[0].Kind != "promise" {
		t.Error("assistant promise should carry")
	}
	if c := ParseCarry(`[{"kind":"deadline","text":"Rocket due Friday"},{"kind":"weird","text":"x y"}]`, 2, 9); len(c) != 2 || c[1].Kind != "important" {
		t.Errorf("ParseCarry: %+v", c)
	}
}

// ---------- token budget ----------

func longTopicChat(nTopics, turnsEach, words int) []llm.Message {
	subjects := []string{"pizza dough", "model rocket", "garden tomatoes", "tax return", "guitar chords", "python script", "wedding speech", "bike repair"}
	var h []llm.Message
	for r := 0; r < 2; r++ { // each topic twice: away and back
		for tp := 0; tp < nTopics; tp++ {
			sub := subjects[tp%len(subjects)]
			fill := strings.Repeat(sub+" ", words)
			for k := 0; k < turnsEach; k++ {
				h = append(h, u("Tell me about the "+sub+" please "+fmt.Sprint(k)+" "+fill), a("Sure, "+fill))
			}
		}
	}
	return append(h, u("back to the pizza dough: what next?"))
}

func TestBudgetIsRespected(t *testing.T) {
	h := longTopicChat(8, 10, 60)
	if Tokens(h) < 40000 {
		t.Fatalf("test chat too small: %d", Tokens(h))
	}
	s := New()
	s.Segment(h, SegOpts{JudgeFrom: len(h)})
	if len(s.Topics) != 8 {
		t.Fatalf("want 8 topics, got %d", len(s.Topics))
	}
	for _, budget := range []int{2000, 6000, 12000} {
		msgs, extra := s.Assemble(h, Opts{Budget: budget})
		if got := Tokens(msgs) + len(extra)/4; got > budget {
			t.Errorf("budget %d: assembled %d tokens", budget, got)
		}
		if msgs[len(msgs)-1].Content != h[len(h)-1].Content {
			t.Errorf("budget %d: the current message must always be sent", budget)
		}
		if msgs[0].Role != "user" {
			t.Errorf("budget %d: must start on a user turn", budget)
		}
		for _, m := range msgs {
			if strings.Contains(m.Content, "model rocket") || strings.Contains(m.Content, "bike repair") {
				t.Fatalf("budget %d: another topic leaked", budget)
			}
		}
	}
	// The topic text never takes more than half the budget, even with many topics.
	many := New()
	for i := 0; i < 60; i++ {
		many.Topics = append(many.Topics, &Topic{ID: i + 1, Title: fmt.Sprintf("Topic number %d with a long title", i), Turns: 3, Last: i, Terms: map[string]float64{"alpha": 1, "beta": 1}})
		many.Carry = append(many.Carry, Carry{Topic: i + 1, Kind: "deadline", Text: strings.Repeat("due by Friday ", 14)})
	}
	many.Active = 1
	_, extra := many.Assemble([]llm.Message{u("hello there friend")}, Opts{Budget: 3000})
	if len(extra)/4 > 1500 {
		t.Errorf("topic text %d tokens over half the budget", len(extra)/4)
	}
}

func TestFoldIsIncremental(t *testing.T) {
	h := longTopicChat(1, 20, 5) // one topic, 80 messages, then the question
	s := New()
	s.Segment(h, SegOpts{})
	id := s.Active
	chunk, start, upto, ok := s.FoldChunk(h, id, 12000)
	if !ok || len(chunk) == 0 || start != 0 || upto <= start {
		t.Fatalf("fold should trigger: ok=%v len=%d start=%d upto=%d", ok, len(chunk), start, upto)
	}
	note := HeuristicNote("", chunk, start)
	if !strings.HasPrefix(note, "Goal: Tell me about the pizza dough") {
		t.Errorf("note goal: %q", note[:60])
	}
	s.SetNote(id, note, upto)
	if _, _, _, again := s.FoldChunk(h, id, 12000); again {
		t.Error("nothing new to fold right after folding")
	}
	msgs, extra := s.Assemble(h, Opts{Budget: 12000})
	if !strings.Contains(extra, "STATE NOTE") {
		t.Error("the note should be in the topic text")
	}
	if len(msgs) > KeepTail+FoldSlack+2 {
		t.Errorf("after the fold only the tail goes word for word, got %d", len(msgs))
	}
	// Updated, never redone: a second fold extends the first note.
	h2 := append(h, a("ok"))
	for k := 0; k < 12; k++ {
		h2 = append(h2, u(fmt.Sprintf("pizza dough follow-up %d", k)), a("sure"))
	}
	s.Segment(h2, SegOpts{})
	c2, st2, up2, ok := s.FoldChunk(h2, id, 12000)
	if !ok {
		t.Fatal("second fold should trigger")
	}
	n2 := HeuristicNote(s.Get(id).Note, c2, st2)
	if !strings.HasPrefix(n2, "Goal: Tell me about the pizza dough") || up2 <= upto || len([]rune(n2)) > NoteMax {
		t.Errorf("second fold should extend the note (upto %d→%d, %d chars)", upto, up2, len([]rune(n2)))
	}
}
