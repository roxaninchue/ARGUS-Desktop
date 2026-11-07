package flashcards

import (
	"testing"
	"time"
)

func TestAddDueReview(t *testing.T) {
	t.Setenv("ARGUS_HOME", t.TempDir())
	now := time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)
	Now = func() time.Time { return now }
	defer func() { Now = time.Now }()
	n, err := Add("NET103", "lecture-4.pdf", [][2]string{{"What layer is IP?", "Network (layer 3)"}, {"Port of HTTPS?", "443"}, {"What layer is IP?", "dup"}, {"", "no front"}})
	if err != nil || n != 2 {
		t.Fatal(n, err)
	}
	ds := Decks()
	if len(ds) != 1 || ds[0].Name != "NET103" || ds[0].Due != 2 || ds[0].New != 2 {
		t.Fatalf("%+v", ds)
	}
	due := Due("NET103", 10)
	if len(due) != 2 {
		t.Fatal(due)
	}
	c, err := Review(due[0].ID, 2)
	if err != nil || c.Interval != 1 || !c.Due.Equal(now.AddDate(0, 0, 1)) {
		t.Fatalf("good on a new card: %+v %v", c, err)
	}
	c, _ = Review(due[1].ID, 0)
	if c.Interval != 0 || c.Lapses != 1 || c.Due.Sub(now) != 10*time.Minute {
		t.Fatalf("again: %+v", c)
	}
	if len(Due("NET103", 10)) != 0 {
		t.Fatal("nothing due right after review")
	}
	now = now.AddDate(0, 0, 1)
	c, _ = Review(due[0].ID, 2)
	if c.Interval != 3 {
		t.Fatalf("second good: %d", c.Interval)
	}
	now = now.AddDate(0, 0, 3)
	c, _ = Review(due[0].ID, 3)
	if c.Interval < 8 || c.Ease <= 2.5 {
		t.Fatalf("easy grows the interval and ease: %+v", c)
	}
	if _, err := Review(due[0].ID, 7); err == nil {
		t.Fatal("bad grade")
	}
	if err := Delete(due[1].ID); err != nil || Decks()[0].Cards != 1 {
		t.Fatal("delete")
	}
}
