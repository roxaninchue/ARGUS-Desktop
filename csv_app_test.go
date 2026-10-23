package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/lacksonkapapahue/blaze-desktop/internal/tools"
)

func cafeCopy(t *testing.T) (dir, src string) {
	t.Helper()
	b, err := os.ReadFile("internal/csvkit/testdata/cafe-sales-sept.csv")
	if err != nil {
		t.Fatal(err)
	}
	dir = t.TempDir()
	src = filepath.Join(dir, "cafe-sales-sept.csv")
	if err := os.WriteFile(src, b, 0o644); err != nil {
		t.Fatal(err)
	}
	return
}

func TestCSVReportToolAndChartURL(t *testing.T) {
	dir, src := cafeCopy(t)
	asked := 0
	e := &tools.Env{Root: dir, Confirm: func(string, string) bool { asked++; return true }}
	out := tools.Run(e, "csv_report", `{"path":"cafe-sales-sept.csv"}`)
	for _, w := range []string{"Total by Store:", "- Riverside: sum 501.30, count 20, mean 25.07", "- Total: sum 1,408.20, count 55", "cafe-sales-sept-clean.csv", "cafe-sales-sept-report.pdf", "Report id: ", "csv_summary"} {
		if !strings.Contains(out, w) {
			t.Errorf("tool output lacks %q:\n%s", w, out)
		}
	}
	if asked != 0 {
		t.Errorf("asked %d times for a file inside the project", asked)
	}
	for _, f := range []string{"cafe-sales-sept-clean.csv", "cafe-sales-sept-chart.svg", "cafe-sales-sept-chart.png", "cafe-sales-sept-report.md", "cafe-sales-sept-report.pdf"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Errorf("missing %s", f)
		}
	}
	url := regexp.MustCompile(`\(/csvkit/chart/[0-9a-f]{16}\.png\)`).FindString(out)
	if url == "" {
		t.Fatalf("no chart url in:\n%s", out)
	}
	h := csvChartHandler(http.NotFoundHandler())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", url[1:len(url)-1], nil))
	if rec.Code != 200 || !strings.HasPrefix(rec.Body.String(), "\x89PNG") {
		t.Errorf("chart url served %d %q", rec.Code, rec.Body.String()[:8])
	}
	for _, bad := range []string{"/csvkit/chart/../../etc/passwd", "/csvkit/chart/0000000000000000.png", "/csvkit/chart/" + filepath.Base(src)} {
		rec = httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", bad, nil))
		if rec.Code != 404 {
			t.Errorf("%s served %d", bad, rec.Code)
		}
	}
	// A different view, and a bad column.
	out = tools.Run(e, "csv_report", `{"path":"cafe-sales-sept.csv","group_by":"Item","agg":"count"}`)
	if !strings.Contains(out, "Count of Total by Item:") {
		t.Errorf("count by item:\n%s", out)
	}
	out = tools.Run(e, "csv_report", `{"path":"cafe-sales-sept.csv","group_by":"Region"}`)
	if !strings.HasPrefix(out, "Error: there's no column \"Region\"") {
		t.Errorf("bad column: %s", out)
	}
}

func TestCSVReportAsksOutsideProject(t *testing.T) {
	_, src := cafeCopy(t)
	e := &tools.Env{Root: t.TempDir(), Confirm: func(string, string) bool { return false }}
	out := tools.Run(e, "csv_report", `{"path":"`+filepath.ToSlash(src)+`"}`)
	if !strings.Contains(out, "did NOT approve") {
		t.Errorf("wrote outside the project without asking: %s", out)
	}
	if _, err := os.Stat(strings.TrimSuffix(src, ".csv") + "-clean.csv"); err == nil {
		t.Error("cleaned file written anyway")
	}
}

func TestCSVReportUIAction(t *testing.T) {
	_, src := cafeCopy(t)
	a := &App{}
	r := a.CSVReport(CSVRequest{Path: src, GroupBy: "store"})
	if r.Error != "" || r.Report == nil || r.Report.Summary.Total.Sum != 1408.20 || !strings.HasPrefix(r.ChartURL, "/csvkit/chart/") {
		t.Fatalf("%+v", r)
	}
	if !strings.Contains(r.Report.ChartSVG, "<svg") || len(r.Report.Preview) != 55 {
		t.Errorf("svg/preview: %d rows", len(r.Report.Preview))
	}
	if msg := a.CSVOpen("/etc/passwd"); msg == "" {
		t.Error("CSVOpen opened a file the report didn't write")
	}
	if r := a.CSVReport(CSVRequest{Path: filepath.Join(filepath.Dir(src), "nope.csv")}); r.Error == "" {
		t.Error("no error for a missing file")
	}
}

// The model's written summary reaches the report: csv_summary fills the Markdown slot and the
// PDF is rewritten with it.
func TestCSVSummaryFillsReport(t *testing.T) {
	dir, _ := cafeCopy(t)
	e := &tools.Env{Root: dir, Confirm: func(string, string) bool { return true }}
	out := tools.Run(e, "csv_report", `{"path":"cafe-sales-sept.csv"}`)
	id := regexp.MustCompile(`Report id: ([0-9a-f]{16})\.`).FindStringSubmatch(out)
	if id == nil {
		t.Fatalf("no report id in:\n%s", out)
	}
	md := filepath.Join(dir, "cafe-sales-sept-report.md")
	pdf := filepath.Join(dir, "cafe-sales-sept-report.pdf")
	before, _ := os.ReadFile(pdf)
	text := "Riverside led September with 501.30 of the 1,408.20 total."
	got := tools.Run(e, "csv_summary", `{"report":"`+id[1]+`","summary":"`+text+`"}`)
	if !strings.HasPrefix(got, "Saved the summary into") {
		t.Fatalf("csv_summary: %s", got)
	}
	b, _ := os.ReadFile(md)
	m := regexp.MustCompile(`(?s)<!-- blaze:summary -->\s*(.*?)\s*<!-- /blaze:summary -->`).FindStringSubmatch(string(b))
	if m == nil || m[1] != text {
		t.Errorf("markdown summary slot = %q", m)
	}
	after, _ := os.ReadFile(pdf)
	if string(after) == string(before) || !strings.Contains(string(after), "Riverside led September") {
		t.Errorf("PDF not rewritten with the summary")
	}
	if out := os.Getenv("CSV_EXPORT_DIR"); out != "" { // for checking the PDF by eye (pdftoppm)
		for _, f := range []string{md, pdf, filepath.Join(dir, "cafe-sales-sept-chart.png")} {
			b, _ := os.ReadFile(f)
			_ = os.WriteFile(filepath.Join(out, filepath.Base(f)), b, 0o644)
		}
	}
	if out := tools.Run(e, "csv_summary", `{"report":"0000000000000000","summary":"x"}`); !strings.HasPrefix(out, "Error: no CSV report") {
		t.Errorf("unknown id: %s", out)
	}
	if out := tools.Run(e, "csv_summary", `{"report":"`+id[1]+`","summary":"  "}`); !strings.HasPrefix(out, "Error: the summary is empty") {
		t.Errorf("empty summary: %s", out)
	}
}
