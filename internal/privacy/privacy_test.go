package privacy

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestScrubNames(t *testing.T) {
	for _, in := range []string{
		"Groq didn't answer, so Cerebras did", "llama-3.3-70b-versatile", "openrouter/deepseek/deepseek-chat-v3:free",
		"SambaNova rate limited: Meta-Llama-3.3-70B-Instruct", "gpt-4o-mini via OpenAI", "Claude 3.5 Sonnet (Anthropic)",
		"Gemini 2.5 Flash", "google/gemma-3-27b-it", "z.ai glm-4.6", "Kimi K2 from Moonshot", "Qwen3-235B", "NVIDIA NIM",
		"Cohere command-r-plus", "Mistral Small", "HuggingFace router", "Hugging Face", "nemotron-70b",
	} {
		out := Scrub(in)
		if Leaks(out) {
			t.Errorf("Scrub(%q) = %q still leaks %v", in, out, Found(out))
		}
		if !Leaks(in) {
			t.Errorf("Leaks(%q) = false", in)
		}
	}
	for _, clean := range []string{"Blaze Cloud", "Sign in to Google again", "Your Google Calendar is connected", "Reading your CSV",
		"Searching the web", "glimpse", "nimble", "The model is busy. Try again in a minute."} {
		if Leaks(clean) || Scrub(clean) != clean {
			t.Errorf("false positive on %q: %v", clean, Found(clean))
		}
	}
}

func TestEventDropsProviderModel(t *testing.T) {
	t.Setenv("BLAZE_DEV", "")
	t.Setenv("BLAZE_DEBUG", "")
	ev := map[string]any{"type": "done", "reply": "Here's what I found about Llama models.", "provider": "groq", "model": "llama-3.3-70b",
		"route": "Groq · llama-3.3-70b", "note": "Cerebras didn't answer, so Groq did",
		"receipt": map[string]any{"summary": "ok"}, "steps": []any{map[string]any{"provider": "cohere", "model": "command-r", "error": "Cohere 429"}}}
	out := Event("done", ev)
	b, _ := json.Marshal(out)
	s := string(b)
	// the reply text is the model's answer and stays (the user asked about Llama)
	if !strings.Contains(s, "about Llama models") {
		t.Fatalf("reply text must stay: %s", s)
	}
	m := out.(map[string]any)
	delete(m, "reply")
	b, _ = json.Marshal(m)
	if Leaks(string(b)) {
		t.Fatalf("chrome leaks %v: %s", Found(string(b)), b)
	}
	if Event("delta", "I use GPT sometimes") != "I use GPT sometimes" {
		t.Fatal("delta must not be touched")
	}
	if Event("status", "Asking Groq") != "Asking the model" {
		t.Fatalf("status: %v", Event("status", "Asking Groq"))
	}
	f := Event("blaze_msg", map[string]any{"kind": "failed", "text": "Couldn't finish: SambaNova is busy", "provider": "x"}).(map[string]any)
	if Leaks(f["text"].(string)) || f["provider"] != nil {
		t.Fatalf("failed final leaks: %v", f)
	}
}

func TestDevFlagShowsNames(t *testing.T) {
	t.Setenv("BLAZE_DEV", "1")
	if Event("status", "Asking Groq") != "Asking Groq" {
		t.Fatal("dev flag should let names through")
	}
}
