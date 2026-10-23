package csvkit

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"flag"
	"image/png"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var update = flag.Bool("update", false, "rewrite golden files")

var fixedNow = time.Date(2026, 10, 22, 18, 0, 0, 0, time.UTC)

func cafe(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/cafe-sales-sept.csv")
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func golden(t *testing.T, name string, got []byte) {
	t.Helper()
	p := filepath.Join("testdata", name)
	if *update {
		if err := os.WriteFile(p, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("%v (run with -update to create it)", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s differs from golden:\n--- got ---\n%s", name, got)
	}
}

func TestCafeCleanedGolden(t *testing.T) {
	r, err := Analyze(cafe(t), "cafe-sales-sept.csv", Options{Now: fixedNow})
	if err != nil {
		t.Fatal(err)
	}
	golden(t, "cafe-sales-sept-clean.golden.csv", r.CleanedCSV())
	st := r.Stats
	if st.RowsIn != 61 || st.RowsOut != 55 || st.Duplicates != 6 {
		t.Errorf("rows in/out/dups = %d/%d/%d, want 61/55/6", st.RowsIn, st.RowsOut, st.Duplicates)
	}
	if st.FillRule != "Total = Qty × Unit Price" {
		t.Errorf("fill rule %q", st.FillRule)
	}
	types := map[string]string{"Date": TypeDate, "Store": TypeCategory, "Item": TypeCategory, "Qty": TypeNumber, "Unit Price": TypeNumber, "Total": TypeNumber, "Payment": TypeCategory}
	for _, c := range r.Columns {
		if types[c.Name] != c.Type {
			t.Errorf("column %s typed %s, want %s", c.Name, c.Type, types[c.Name])
		}
	}
	if c := r.Columns[0]; c.DateOrder != "DMY+MDY" {
		t.Errorf("date order %q", c.DateOrder)
	}
	// Every cleaned date is ISO and in September 2026; every store spelling is canonical.
	for _, row := range r.Table().Rows {
		if !strings.HasPrefix(row[0], "2026-09-") || len(row[0]) != 10 {
			t.Errorf("date %q", row[0])
		}
		switch row[1] {
		case "Riverside", "Old Town", "Campus", "":
		default:
			t.Errorf("store %q", row[1])
		}
		if row[5] == "" {
			t.Errorf("a Total is still blank: %v", row)
		}
	}
}

func TestCafeRevenueByStore(t *testing.T) {
	r, err := Analyze(cafe(t), "cafe-sales-sept.csv", Options{Now: fixedNow})
	if err != nil {
		t.Fatal(err)
	}
	s := r.Summary
	if s.By != "Store" || s.Value != "Total" || s.Agg != "sum" {
		t.Fatalf("picked %s by %s (%s)", s.Value, s.By, s.Agg)
	}
	// Reference values from an independent script (work/w2_expected.py).
	want := []struct {
		key   string
		sum   float64
		count int
	}{{"Riverside", 501.30, 20}, {"Campus", 352.40, 15}, {"Old Town", 345.90, 14}, {Blank, 208.60, 6}}
	if len(s.Groups) != len(want) {
		t.Fatalf("groups %+v", s.Groups)
	}
	for i, w := range want {
		g := s.Groups[i]
		if g.Key != w.key || g.Sum != w.sum || g.Count != w.count {
			t.Errorf("group %d = %s %.2f %d, want %s %.2f %d", i, g.Key, g.Sum, g.Count, w.key, w.sum, w.count)
		}
	}
	if s.Total.Sum != 1408.20 || s.Total.Count != 55 {
		t.Errorf("total %.2f/%d", s.Total.Sum, s.Total.Count)
	}
	if got := s.Fmt(s.Groups[0].Mean, "mean"); got != "25.07" {
		t.Errorf("Riverside mean %s", got)
	}
	// Other aggregates and groupings.
	tb := r.Table()
	m, err := tb.GroupBy(tb.Index("item"), tb.Index("qty"), "mean")
	if err != nil || m.Agg != "mean" || len(m.Groups) != 8 {
		t.Fatalf("mean by item: %v %+v", err, m)
	}
	c, _ := tb.GroupBy(tb.Index("payment"), -1, "")
	if c.Agg != "count" || c.Total.Rows != 55 {
		t.Errorf("count by payment %+v", c)
	}
	d, _ := tb.GroupBy(0, tb.Index("total"), "sum")
	if !d.ByDate || d.Groups[0].Key != "2026-09-01" || NewChart(d, "").Kind != Line {
		t.Errorf("by date: %+v", d.Groups[:2])
	}
}

func TestChartSVGWellFormed(t *testing.T) {
	r, err := Analyze(cafe(t), "cafe-sales-sept.csv", Options{Now: fixedNow})
	if err != nil {
		t.Fatal(err)
	}
	tb := r.Table()
	d, _ := tb.GroupBy(0, tb.Index("total"), "sum")
	for name, svg := range map[string]string{"bar": r.ChartSVG, "line": NewChart(d, "").SVG()} {
		dec := xml.NewDecoder(strings.NewReader(svg))
		n, texts := 0, []string{}
		inText := false
		for {
			tok, err := dec.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("%s svg not well-formed: %v", name, err)
			}
			switch x := tok.(type) {
			case xml.StartElement:
				n++
				if n == 1 && x.Name.Local != "svg" {
					t.Fatalf("root %s", x.Name.Local)
				}
				inText = x.Name.Local == "text"
			case xml.CharData:
				if inText {
					texts = append(texts, string(x))
				}
			case xml.EndElement:
				inText = false
			}
		}
		all := strings.Join(texts, "|")
		if name == "bar" {
			for _, w := range []string{"Total by Store", "Riverside", "Old Town", "Campus", "(blank)", "501.30", "208.60"} {
				if !strings.Contains(all, w) {
					t.Errorf("bar svg lacks %q: %s", w, all)
				}
			}
		} else if !strings.Contains(svg, "<polyline") {
			t.Error("line svg has no polyline")
		}
	}
}

func TestRunWritesFiles(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "cafe-sales-sept.csv")
	if err := os.WriteFile(src, cafe(t), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := Run(src, Options{Now: fixedNow})
	if err != nil {
		t.Fatal(err)
	}
	if r.Files.Cleaned != filepath.Join(dir, "cafe-sales-sept-clean.csv") {
		t.Errorf("cleaned at %s", r.Files.Cleaned)
	}
	if b, _ := os.ReadFile(src); !bytes.Equal(b, cafe(t)) {
		t.Error("source changed")
	}
	cl, _ := os.ReadFile(r.Files.Cleaned)
	if !bytes.Equal(cl, r.CleanedCSV()) {
		t.Error("cleaned file differs")
	}
	md, _ := os.ReadFile(r.Files.MD)
	for _, w := range []string{SummaryMarker, SummaryEndMarker, SummaryPending, "![Total by Store](cafe-sales-sept-chart.png)", "| Riverside | 501.30 | 20 | 25.07 |", "**1,408.20**", "cafe-sales-sept-chart.svg"} {
		if !bytes.Contains(md, []byte(w)) {
			t.Errorf("markdown lacks %q", w)
		}
	}
	golden(t, "cafe-sales-sept-report.golden.md", md)
	pb, _ := os.ReadFile(r.Files.PNG)
	img, err := png.Decode(bytes.NewReader(pb))
	if err != nil || img.Bounds().Dx() != 1520 || img.Bounds().Dy() != 840 {
		t.Fatalf("png: %v %v", err, img.Bounds())
	}
	pdf, _ := os.ReadFile(r.Files.PDF)
	if !bytes.HasPrefix(pdf, []byte("%PDF-1.4")) || !bytes.HasSuffix(pdf, []byte("%%EOF\n")) {
		t.Fatalf("pdf header/trailer: %q", pdf[:20])
	}
	if pt, err := exec.LookPath("pdftotext"); err == nil {
		out, err := exec.Command(pt, "-layout", r.Files.PDF, "-").Output()
		if err != nil {
			t.Fatalf("pdftotext: %v", err)
		}
		for _, w := range []string{"501.30", "352.40", "345.90", "208.60", "1,408.20", "Riverside", "Cleaned data", SummaryPending[:40]} {
			if !bytes.Contains(out, []byte(w)) {
				t.Errorf("pdf text lacks %q", w)
			}
		}
	} else {
		t.Log("pdftotext not installed; skipped the PDF text check")
	}
	if os.Getenv("CSVKIT_KEEP") != "" { // for looking at the outputs by eye
		k := os.Getenv("CSVKIT_KEEP")
		_ = os.MkdirAll(k, 0o755)
		for _, f := range []string{r.Files.Cleaned, r.Files.SVG, r.Files.PNG, r.Files.MD, r.Files.PDF} {
			b, _ := os.ReadFile(f)
			_ = os.WriteFile(filepath.Join(k, filepath.Base(f)), b, 0o644)
		}
		js, _ := json.Marshal(map[string]any{"report": r, "chart_url": ""})
		_ = os.WriteFile(filepath.Join(k, "report.json"), js, 0o644)
		tb := r.Table()
		d, _ := tb.GroupBy(0, tb.Index("total"), "sum")
		lp, _ := NewChart(d, "").PNG(2)
		_ = os.WriteFile(filepath.Join(k, "line-by-date.png"), lp, 0o644)
		it, _ := tb.GroupBy(tb.Index("item"), tb.Index("total"), "sum")
		ip, _ := NewChart(it, "").PNG(2)
		_ = os.WriteFile(filepath.Join(k, "bar-by-item.png"), ip, 0o644)
	}
	// The summary slot fills in and the files are rewritten.
	if err := r.SetSummary("Riverside led September."); err != nil {
		t.Fatal(err)
	}
	md, _ = os.ReadFile(r.Files.MD)
	if !bytes.Contains(md, []byte(SummaryMarker+"\nRiverside led September.\n"+SummaryEndMarker)) || bytes.Contains(md, []byte(SummaryPending)) {
		t.Errorf("summary not slotted in:\n%s", md[:400])
	}
}

func TestParseRobust(t *testing.T) {
	// BOM, semicolons, quoted delimiter and newline, ragged rows, blank lines, repeated header,
	// European numbers.
	in := "\xef\xbb\xbfName;Amount;Note\n" +
		"\"Smith; J\";\"1.234,50\";\"two\nlines\"\n" +
		"\n" +
		"Doe;12,5\n" +
		"Name;Amount;Note\n" +
		"Roe;€7;x;extra\n" +
		" ; ;\n"
	raw, err := Parse([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	if raw.Delimiter != ';' || raw.BOM != "utf-8" {
		t.Errorf("delim %q bom %q", raw.Delimiter, raw.BOM)
	}
	if len(raw.Header) != 4 || raw.Header[3] != "Column 4" || raw.Ragged != 2 {
		t.Errorf("header %v ragged %d", raw.Header, raw.Ragged)
	}
	tb := Clean(raw)
	if tb.Stats.HeaderRepeats != 1 || tb.Stats.BlankRows != 1 || len(tb.Rows) != 3 {
		t.Fatalf("stats %+v rows %v", tb.Stats, tb.Rows)
	}
	if tb.Rows[0][0] != "Smith; J" || tb.Rows[0][2] != "two lines" {
		t.Errorf("quoted row %v", tb.Rows[0])
	}
	if tb.Columns[1].Type != TypeNumber || tb.Rows[0][1] != "1234.50" || tb.Rows[1][1] != "12.50" || tb.Rows[2][1] != "7.00" {
		t.Errorf("amounts %v %v %v (%s)", tb.Rows[0][1], tb.Rows[1][1], tb.Rows[2][1], tb.Columns[1].Type)
	}
	// UTF-16 with a BOM and tabs.
	u := []byte{0xFF, 0xFE}
	for _, r := range "a\tb\n1\t2\n" {
		u = append(u, byte(r), 0)
	}
	raw, err = Parse(u)
	if err != nil || raw.Delimiter != '\t' || raw.BOM != "utf-16le" || raw.Rows[0][1] != "2" {
		t.Errorf("utf-16: %v %+v", err, raw)
	}
}

func TestParseNumber(t *testing.T) {
	cases := map[string]float64{"1,234.56": 1234.56, "$3.40": 3.4, "(12)": -12, "-4%": -4, "1 200": 1200, "1.234.567": 1234567,
		"3e2": 300, "€1.234,5": 1234.5, "USD 10": 10, "12,5": 12.5, "-$3.40": -3.4, "£0.99": 0.99}
	for in, want := range cases {
		n, ok := ParseNumber(in)
		if !ok || n.Value != want {
			t.Errorf("ParseNumber(%q) = %v %v, want %v", in, n.Value, ok, want)
		}
	}
	for _, bad := range []string{"", "abc", "12a", "1,2,3", "e5", "2026-09-01"} {
		if _, ok := ParseNumber(bad); ok {
			t.Errorf("ParseNumber(%q) accepted", bad)
		}
	}
}

func TestDates(t *testing.T) {
	out, order, _, bad := normaliseDates([]string{"03/04/2026", "04/05/2026", "Sep 5, 2026", "5 Sept 2026", "2026-09-05T10:30:00Z", "31/12/2026", "nope", ""})
	want := []string{"2026-04-03", "2026-05-04", "2026-09-05", "2026-09-05", "2026-09-05 10:30:00", "2026-12-31", "nope", ""}
	for i := range want {
		if out[i] != want[i] {
			t.Errorf("date %d = %q, want %q", i, out[i], want[i])
		}
	}
	if order != "DMY" || bad != 1 {
		t.Errorf("order %s bad %d", order, bad)
	}
	// No cell settles it: month-first.
	out, order, _, _ = normaliseDates([]string{"03/04/2026"})
	if out[0] != "2026-03-04" || order != "MDY" {
		t.Errorf("ambiguous: %v %s", out, order)
	}
}
