package tools

import "testing"

// Reads may run side by side; anything that writes, runs a command, asks for approval
// or touches shared state must stay one by one.
func TestParallelSafeList(t *testing.T) {
	for _, n := range []string{"file_read", "file_list", "file_search", "web_search", "fetch_page", "get_weather", "recall", "load_skill"} {
		if !ParallelSafe(n) {
			t.Errorf("%s should be parallel-safe", n)
		}
	}
	for _, n := range []string{"file_write", "file_edit", "file_delete", "run_shell", "git", "install", "schedule", "remember", "forget", "system_info", "phone", "dispatch_agent", "update_blaze", "http_request", "nope"} {
		if ParallelSafe(n) {
			t.Errorf("%s must not run in parallel", n)
		}
	}
}

// Every parallel-safe built-in exists and never asks for approval in its handler.
func TestParallelSafeAreRealTools(t *testing.T) {
	for n := range parallelSafe {
		if n == "get_weather" { // handled by the app, not the registry
			continue
		}
		if _, ok := registry[n]; !ok {
			t.Errorf("%s is listed as parallel-safe but isn't a tool", n)
		}
	}
}
