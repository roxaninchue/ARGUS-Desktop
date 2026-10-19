package home

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseIntent(t *testing.T) {
	cases := []struct{ in, typ, title, key, val string }{
		{"weather in lusaka", Weather, "Weather in Lusaka", "city", "Lusaka"},
		{"weather", Weather, "Weather", "use_location", "true"},
		{"track bitcoin price", Prices, "Bitcoin", "ids", "bitcoin"},
		{"price of eth", Prices, "Ethereum", "ids", "ethereum"},
		{"usd to zmw", Prices, "USD to ZMW", "symbols", "ZMW"},
		{"AAPL stock", Prices, "AAPL stock", "symbols", "AAPL"},
		{"Arsenal scores", Sports, "Arsenal", "team", "Arsenal"},
		{"results for the Lakers", Sports, "Lakers", "team", "Lakers"},
		{"github.com/golang/go repo", GitHub, "golang/go", "repo", "golang/go"},
		{"my deadlines", Deadlines, "Deadlines", "", ""},
		{"my reminders", Schedule, "Schedule", "", ""},
		{"news about AI chips", News, "AI Chips news", "query", "AI chips"},
		{"formula 1 news", News, "Formula 1 news", "query", "formula 1"},
		{"headlines", News, "Top stories", "query", ""},
		{"https://blog.golang.org/feed.atom/feed", News, "Feed", "feed", "https://blog.golang.org/feed.atom/feed"},
	}
	for _, c := range cases {
		s, ok := Parse(c.in)
		if !ok || s.Type != c.typ || s.Title != c.title || (c.key != "" && s.Config[c.key] != c.val) {
			t.Errorf("Parse(%q) = %+v %v", c.in, s, ok)
		}
	}
	if _, ok := Parse("write me a poem"); ok {
		t.Error("a poem isn't a widget")
	}
}

func TestStacksAndStore(t *testing.T) {
	s := Open(filepath.Join(t.TempDir(), "home.json"))
	if !s.Seed(Starters()) || s.Seed(Starters()) {
		t.Fatal("seed once")
	}
	n := len(s.All())
	if n != len(Starters()) {
		t.Fatalf("starters %d", n)
	}
	a := s.Add(Crypto("btc").Widget())
	b := s.Add(Crypto("bitcoin").Widget()) // same config: no duplicate
	if a.ID != b.ID || len(s.All()) != n+1 {
		t.Fatal("duplicate widget")
	}
	s.Add(Crypto("eth").Widget())
	st := Stacks(s.All())
	var fin *Stack
	for i := range st {
		if st[i].Category == "Finance" {
			fin = &st[i]
		}
	}
	if fin == nil || len(fin.Widgets) != 2 {
		t.Fatalf("finance stack %+v", st)
	}
	if !s.Move(a.ID, -1) || s.All()[n-1].ID != a.ID {
		t.Fatal("move up")
	}
	if s.Move(s.All()[0].ID, -1) {
		t.Fatal("can't move the first one up")
	}
	// A failed fetch keeps the last good content.
	now := time.Now()
	s.SaveData(a.ID, Data{Value: "1 USD"}, now)
	d := s.SaveData(a.ID, Failed("No connection right now.", ""), now.Add(time.Hour))
	if d.Value != "1 USD" || d.Error == "" || d.Fetched != now.UnixMilli() {
		t.Fatalf("kept %+v", d)
	}
	// Reopen from disk.
	s2 := Open(s.path)
	if len(s2.All()) != len(s.All()) || s2.Snapshot().Data[a.ID].Value != "1 USD" {
		t.Fatal("not saved")
	}
	s2.Remove(a.ID)
	if _, ok := s2.Get(a.ID); ok || len(s2.RemovedCategories()) != 1 {
		t.Fatal("remove")
	}
	ids := []string{}
	for _, w := range s2.All() {
		ids = append([]string{w.ID}, ids...)
	}
	s2.Reorder(ids)
	if s2.All()[0].ID != ids[0] {
		t.Fatal("reorder")
	}
}

const rss = `<?xml version="1.0"?><rss><channel><title>Google News</title>
<item><title>Big story happens today in Lusaka - The Times</title><link>https://example.com/a</link><source url="x">The Times</source><pubDate>Mon, 19 Oct 2026 10:00:00 GMT</pubDate></item>
<item><title><![CDATA[Second &amp; story]]></title><link>https://example.com/b</link><pubDate>Mon, 19 Oct 2026 08:00:00 GMT</pubDate><media:thumbnail url="http://img.example.com/b.jpg"/></item>
</channel></rss>`

const bing = `<rss><channel><title>Bing</title><item><title>Big story happens today in Lusaka</title><News:Image>https://www.bing.com/th?id=OVF.x</News:Image></item></channel></rss>`

func TestFeeds(t *testing.T) {
	es := ParseFeed(rss, 10)
	if len(es) != 2 || es[0].Title != "Big story happens today in Lusaka" || es[0].Source != "The Times" {
		t.Fatalf("%+v", es)
	}
	if es[1].Title != "Second & story" || es[1].Image != "https://img.example.com/b.jpg" {
		t.Fatalf("%+v", es[1])
	}
	es = BorrowImages(es, ParseFeed(bing, 10))
	if !strings.HasPrefix(es[0].Image, "https://www.bing.com/th?id=OVF.x&w=480") {
		t.Fatalf("borrow %q", es[0].Image)
	}
	now := ParseDate("Mon, 19 Oct 2026 12:00:00 GMT")
	d := FeedData(es, "Top stories", now)
	if d.Lines[0].S != "The Times · 2 h ago" || FeedTitle(rss) != "Google News" {
		t.Fatalf("%+v", d.Lines[0])
	}
	if !strings.Contains(NewsURL("AI chips", "en", "ZM"), "q=AI+chips&hl=en-ZM&gl=ZM&ceid=ZM:en") {
		t.Fatal(NewsURL("AI chips", "en", "ZM"))
	}
}

const forecast = `{"current_units":{"temperature_2m":"°C","wind_speed_10m":"km/h"},"current":{"temperature_2m":27.6,"apparent_temperature":28.2,"weather_code":2,"wind_speed_10m":11.4},
"daily":{"time":["2026-10-19","2026-10-20"],"weather_code":[2,61],"temperature_2m_max":[31.2,29.5],"temperature_2m_min":[17.4,18.0],"precipitation_probability_max":[0,40]}}`

func TestWeatherAndPrices(t *testing.T) {
	d, err := ParseForecast(forecast, "Lusaka")
	if err != nil || d.Value != "28°C" || d.Change != "Partly cloudy · feels 28° · wind 11 km/h" || len(d.Lines) != 2 || d.Lines[1].S != "Rain · rain 40%" || d.Lines[0].N != "31° / 17°" {
		t.Fatalf("%+v %v", d, err)
	}
	if _, err := ParseForecast(`{"error":true}`, "x"); err == nil {
		t.Fatal("error json")
	}
	c, err := ParseCoins(`{"bitcoin":{"usd":64210.4,"usd_24h_change":-1.23}}`, []string{"bitcoin"}, "usd")
	if err != nil || c.Value != "64,210 USD" || c.Change != "▼ 1.2% today" {
		t.Fatalf("%+v %v", c, err)
	}
	if _, err := ParseCoins(`{}`, []string{"nope"}, "usd"); err == nil {
		t.Fatal("missing coin")
	}
	fx, err := ParseFx(`{"result":"success","rates":{"ZMW":26.5,"EUR":0.92}}`, "usd", []string{"ZMW", "EUR"})
	if err != nil || fx.Value != "26.50 ZMW" || len(fx.Lines) != 2 {
		t.Fatalf("%+v %v", fx, err)
	}
	l, err := ParseStock(`{"chart":{"result":[{"meta":{"symbol":"AAPL","currency":"USD","regularMarketPrice":200,"chartPreviousClose":190}}]}}`, "aapl")
	if err != nil || l.N != "200.00 USD  ▲ 5.3%" {
		t.Fatalf("%+v %v", l, err)
	}
	if Money(0.00123, "USD") != "0.001230 USD" || Pct(2.04) != "▲ 2.0%" {
		t.Fatal(Money(0.00123, "USD"))
	}
}

func TestSportsAndGitHub(t *testing.T) {
	team, ok := ParseTeam(`{"teams":[{"idTeam":"1","strTeam":"Arsenal Tula"},{"idTeam":"2","strTeam":"Arsenal","strLeague":"English Premier League"}]}`, "arsenal")
	if !ok || team.ID != "2" {
		t.Fatalf("%+v", team)
	}
	last := ParseEvents(`{"results":[{"strEvent":"Arsenal vs Chelsea","strHomeTeam":"Arsenal","strAwayTeam":"Chelsea","intHomeScore":"2","intAwayScore":"1","strTimestamp":"2026-10-18T15:00:00+00:00","strStatus":"FT"}]}`, "results")
	next := ParseEvents(`{"events":[{"strHomeTeam":"Spurs","strAwayTeam":"Arsenal","intHomeScore":null,"strTimestamp":"2026-10-25T14:00:00"}]}`, "events")
	d := SportsData(team, last, next, time.UTC)
	if d.Value != "2–1" || len(d.Lines) != 2 || d.Lines[0].N != "Next" || d.Lines[0].S != "Sun 25 Oct · 14:00" || d.Lines[1].T != "Arsenal 2–1 Chelsea" {
		t.Fatalf("%+v", d)
	}
	if Repo("https://github.com/golang/go.git") != "golang/go" || Repo("not a repo") != "" {
		t.Fatal("repo")
	}
	now := ParseDate("2026-10-19T12:00:00Z")
	g, err := ParseRepoEvents(`[{"type":"PushEvent","actor":{"login":"ada"},"created_at":"2026-10-19T11:00:00Z","payload":{"size":2,"commits":[{"message":"a"},{"message":"Fix the build\n\nmore"}]}},
	{"type":"PullRequestEvent","actor":{"login":"bo"},"created_at":"2026-10-19T10:00:00Z","payload":{"action":"opened","pull_request":{"number":7,"title":"Home tab","html_url":"https://github.com/o/r/pull/7"}}},
	{"type":"GollumEvent"}]`, "o/r", now)
	if err != nil || len(g.Lines) != 2 || g.Lines[0].T != "Fix the build" || g.Lines[0].S != "ada · 1 h ago" || g.Lines[1].T != "Opened PR #7: Home tab" {
		t.Fatalf("%+v %v", g, err)
	}
	if _, err := ParseRepoEvents(`{"message":"API rate limit exceeded"}`, "o/r", now); err == nil || !strings.Contains(err.Error(), "free limit") {
		t.Fatal(err)
	}
}

func TestFetcher(t *testing.T) {
	calls := []string{}
	f := Fetcher{Lang: "en", Country: "ZM", Loc: time.UTC, Net: func(_ context.Context, u string, _ map[string]string) (int, string, error) {
		calls = append(calls, u)
		switch {
		case strings.Contains(u, "geocoding"):
			return 200, `{"results":[{"name":"Lusaka","latitude":-15.4,"longitude":28.3,"country":"Zambia"}]}`, nil
		case strings.Contains(u, "api.open-meteo.com"):
			return 200, forecast, nil
		case strings.Contains(u, "news.google.com"):
			return 200, rss, nil
		case strings.Contains(u, "bing.com"):
			return 200, bing, nil
		case strings.Contains(u, "coingecko"):
			return 503, "", nil
		}
		return 0, "", errors.New("dial tcp: no such host")
	}}
	w := Widget{Type: Weather, Title: "Weather", Config: map[string]string{"city": "Lusaka"}}
	r := f.Fetch(context.Background(), w, time.Now())
	if r.Data.Value != "28°C" || r.Data.Headline != "Lusaka" || r.Learned["geo_city"] != "Lusaka" {
		t.Fatalf("%+v", r)
	}
	// Learned coordinates are reused: no second geocode.
	for k, v := range r.Learned {
		w.Config[k] = v
	}
	calls = nil
	f.Fetch(context.Background(), w, time.Now())
	if len(calls) != 1 || !strings.Contains(calls[0], "latitude=-15.4000") {
		t.Fatalf("calls %v", calls)
	}
	n := f.Fetch(context.Background(), Widget{Type: News, Title: "Top stories", Config: map[string]string{}}, time.Now())
	if len(n.Data.Lines) != 2 || n.Data.Lines[0].I == "" {
		t.Fatalf("%+v", n.Data)
	}
	p := f.Fetch(context.Background(), Crypto("btc").Widget(), time.Now())
	if p.Data.Error != "The source is having trouble. I'll try again later." {
		t.Fatalf("%+v", p.Data)
	}
	nc := f.Fetch(context.Background(), Widget{Type: Weather, Config: map[string]string{}}, time.Now())
	if nc.Data.Action != "city" || nc.Data.Error != "Type a city to see the weather." {
		t.Fatalf("%+v", nc.Data)
	}
	g := f.Fetch(context.Background(), Widget{Type: GitHub, Config: map[string]string{"repo": "o/r"}}, time.Now())
	if g.Data.Error != "Couldn't update this right now." {
		t.Fatalf("%+v", g.Data)
	}
	if pl, ok := ParseIPLocate(`{"success":true,"city":"Lusaka","country_code":"ZM","latitude":-15.4,"longitude":28.3}`); !ok || pl.Name != "Lusaka" {
		t.Fatal("ip locate")
	}
}

func TestLocalCards(t *testing.T) {
	now := time.Date(2026, 10, 19, 14, 0, 0, 0, time.UTC)
	jobs := []Job{
		{Name: "standup", When: "every weekday at 9:00", Prompt: "Brief me"},
		{Name: "dentist", When: "once 2026-10-21 10:30", Prompt: "Remind me: dentist"},
		{Name: "old", When: "once 2026-10-01 10:30"},
		{Name: "call mum", When: "once 2026-10-20 18:00"},
	}
	d := ScheduleData(jobs, now)
	if d.Value != "call mum" || d.Change != "Tue 20 Oct · 18:00" || len(d.Lines) != 2 || d.Lines[0].T != "dentist" || d.Lines[1].T != "standup" {
		t.Fatalf("%+v", d)
	}
	if e := ScheduleData(nil, now); !strings.HasPrefix(e.Action, "ask:") {
		t.Fatal("empty schedule asks Blaze")
	}
	dl := DeadlinesData(jobs, []string{"Report is due Friday", "Likes tea"}, now)
	if dl.Value != "call mum" || len(dl.Lines) != 2 || dl.Lines[1].N != "From memory" {
		t.Fatalf("%+v", dl)
	}
	r := RecentsData([]Recent{{Title: "Fix the tests", Sub: "Chat", Kind: "chat", Target: "main"}, {Title: ""}, {Title: "HomeGlow", Kind: "project", Target: "p1"}})
	if len(r.Lines) != 2 || r.Lines[1].U != "blaze:project:p1" {
		t.Fatalf("%+v", r)
	}
	c := CustomData(Widget{Title: "Links", Config: map[string]string{"items": "Docs | read | https://x.dev\nbad | | javascript:alert(1)", "url": "ftp://x"}})
	if len(c.Lines) != 2 || c.Lines[0].U != "https://x.dev" || c.Lines[1].U != "" || c.Link != "" {
		t.Fatalf("%+v", c)
	}
}

func TestGreetingNameMemories(t *testing.T) {
	at := func(h int) time.Time { return time.Date(2026, 1, 1, h, 0, 0, 0, time.UTC) }
	if Greeting(at(6)) != "Good morning" || Greeting(at(13)) != "Good afternoon" || Greeting(at(22)) != "Good evening" || Greeting(at(3)) != "Good evening" {
		t.Fatal("greeting")
	}
	if NameFrom([]string{"Likes Go", "The user's name is Luke and he codes"}) != "Luke" || NameFrom([]string{"no name"}) != "" {
		t.Fatal("name")
	}
	got := FromMemories([]string{"Lives in Lusaka", "Interested in Sports: Arsenal; Formula 1", "Holds some bitcoin"})
	if len(got) < 3 || got[0].Type != Weather || got[0].Config["city"] != "Lusaka" {
		t.Fatalf("%+v", got)
	}
	if FromMemories([]string{"likes tea"}) != nil {
		t.Fatal("not confident")
	}
	if Ago(0, 1) != "Not updated yet" || Ago(1, 1+3*3600*1000) != "Updated 3 h ago" {
		t.Fatal("ago")
	}
	if NormType("crypto") != Prices || NormType("reminders") != Schedule || NormType("x") != "" {
		t.Fatal("types")
	}
}
