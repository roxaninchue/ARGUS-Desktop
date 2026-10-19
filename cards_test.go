package main

import (
	"strings"
	"testing"
)

func TestAnswerShape(t *testing.T) {
	on := answerShape(true)
	for _, want := range []string{":::card", "short answer", "\n:::\n"} {
		if !strings.Contains(on, want) {
			t.Errorf("answerShape(true) is missing %q", want)
		}
	}
	if answerShape(false) != "" {
		t.Error("answerShape(false) should add nothing")
	}
}

func TestFoldCardsDefault(t *testing.T) {
	a := &App{}
	if !a.foldCards() {
		t.Error("fold_cards should default to on")
	}
	off := false
	a.prefs.FoldCards = &off
	if a.foldCards() {
		t.Error("fold_cards=false should turn cards off")
	}
}

func TestReplyExtras(t *testing.T) {
	x := replyExtras()
	for _, want := range []string{"RICH REPLIES", "**bold title**", "```mermaid", "<<follow-ups: first | second | third>>", "never when calling tools"} {
		if !strings.Contains(x, want) {
			t.Errorf("replyExtras() is missing %q", want)
		}
	}
}
