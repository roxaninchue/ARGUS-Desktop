package main

import "testing"

func TestFriendlyTool(t *testing.T) {
	cases := map[[2]string]string{
		{"file_read", `{"path":"src/index.html"}`}:                       "Reading index.html",
		{"web_search", `{"query":"wails tray"}`}:                         `Searching the web for "wails tray"`,
		{"fetch_page", `{"url":"https://www.example.com/a"}`}:            "Reading example.com",
		{"run_shell", `{"command":"pnpm install"}`}:                      "Installing packages",
		{"run_shell", `{"command":"ls -la"}`}:                            "Running a command",
		{"run_code", `{"language":"python"}`}:                            "Running python code",
		{"mystery", `{}`}:                                                "Working on it",
		{"run_shell", `{"command":"x","summary":"checking disk space"}`}: "Checking disk space",
		{"run_shell", `{"command":"x","summary":"rm -rf ~/; echo"}`}:     "Running a command",
		// 0.19.16: the pill names the real object, never a static label
		{"csv_report", `{"path":"/h/cafe.csv","group_by":"store","value":"revenue"}`}:                        "Charting revenue by store",
		{"csv_report", `{"path":"/h/cafe-sales-sept.csv"}`}:                                                  "Making a report from cafe-sales-sept.csv",
		{"csv_summary", `{"report":"r1","summary":"Revenue rose."}`}:                                         "Writing your summary",
		{"get_weather", `{"place":"Lusaka"}`}:                                                                "Checking the weather in Lusaka",
		{"memory_search", `{"query":"sister"}`}:                                                              "Checking what I remember about sister",
		{"schedule_add", `{"text":"call mum","at":"17:00"}`}:                                                 "Setting a reminder to call mum",
		{"dispatch_specialist", `{"name":"build-fixer","brief":"Fix the build error in main.go. Files: x"}`}: "Getting help to fix the build error in main.go",
		{"apply_specialist", `{"name":"build-fixer","action":"apply"}`}:                                      "Bringing in the reviewed changes",
		{"mystery", `{"path":"/a/notes.md"}`}:                                                                "Working on notes.md",
	}
	for in, want := range cases {
		if got := friendlyTool(in[0], in[1]); got != want {
			t.Errorf("%s %s = %q, want %q", in[0], in[1], got, want)
		}
	}
	long := friendlyTool("web_search", `{"query":"a very long query that goes on and on and on past the limit"}`)
	if n := len([]rune(long)); n > 60 {
		t.Errorf("too long: %d", n)
	}
}

func TestParseFlags(t *testing.T) {
	f := parseFlags([]string{"--background", "--orb", "--relaunch"})
	if !f.background || !f.orb || !f.relaunch {
		t.Fatal(f)
	}
}
