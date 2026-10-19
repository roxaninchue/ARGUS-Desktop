package media

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestQueueLogic(t *testing.T) {
	o := ShuffleOrder(5, 3, 42)
	if len(o) != 5 || o[0] != 3 {
		t.Fatalf("shuffle starts with the tapped song: %v", o)
	}
	if !reflect.DeepEqual(ShuffleOrder(5, 3, 42), o) {
		t.Fatal("shuffle is deterministic for a seed")
	}
	if NextIndex(4, 5, RepeatOff, nil) != -1 || NextIndex(4, 5, RepeatAll, nil) != 0 || NextIndex(2, 5, RepeatOne, nil) != 2 {
		t.Fatal("next index with repeat")
	}
	if PrevIndex(0, 5, RepeatOff, nil) != -1 || PrevIndex(0, 5, RepeatAll, nil) != 4 {
		t.Fatal("prev index")
	}
	if got := UpNext(3, 5, RepeatAll, nil, 10); !reflect.DeepEqual(got, []int{4, 0, 1, 2}) {
		t.Fatalf("up next wraps with repeat all: %v", got)
	}
	if got := Move([]string{"a", "b", "c"}, 0, 2); !reflect.DeepEqual(got, []string{"b", "c", "a"}) {
		t.Fatalf("move: %v", got)
	}
	if RepeatOff.Next() != RepeatAll || RepeatOne.Next() != RepeatOff {
		t.Fatal("repeat cycles")
	}
}

func TestPlaylistOps(t *testing.T) {
	l := PlCreate(nil, "  Road   trip ", "a", 1)
	l = PlCreate(l, "road trip", "b", 2)
	if l[0].Name != "Road trip" || l[1].Name != "road trip 2" {
		t.Fatalf("unique names: %+v", l)
	}
	l = PlAdd(l, "a", []string{"x", "y", "x"}, 3)
	if !reflect.DeepEqual(l[0].Keys, []string{"x", "y"}) {
		t.Fatalf("add skips duplicates: %v", l[0].Keys)
	}
	l = PlMove(l, "a", 1, 0, 4)
	l = PlRemoveAt(l, "a", 1, 5)
	if !reflect.DeepEqual(l[0].Keys, []string{"y"}) {
		t.Fatalf("move + remove: %v", l[0].Keys)
	}
	l = PlRename(l, "b", "Road trip", 6)
	if l[1].Name != "Road trip 2" {
		t.Fatalf("rename stays unique: %s", l[1].Name)
	}
	l = PlForget(l, "y")
	if len(l[0].Keys) != 0 || len(PlDelete(l, "a")) != 1 {
		t.Fatal("forget + delete")
	}
	if got := PushRecent([]string{"a", "b", "c"}, "c", 2); !reflect.DeepEqual(got, []string{"c", "a"}) {
		t.Fatalf("recents: %v", got)
	}
	if ResumeAt(3000, 60000) != 0 || ResumeAt(55000, 60000) != 0 || ResumeAt(30000, 60000) != 30000 {
		t.Fatal("resume rules")
	}
}

func songs() []Track {
	return []Track{
		{Key: "1", Title: "Slow Morning", Artist: "Ana", Album: "Calm", Genre: "Lo-Fi", AddedAt: 100, Kind: "audio"},
		{Key: "2", Title: "Party Starter", Artist: "Ben", Genre: "Dance", AddedAt: 200, Kind: "audio"},
		{Key: "3", Title: "Piano Rain", Artist: "Ana", AddedAt: 300, Kind: "audio"},
		{Key: "4", Title: "Gym Hype", Artist: "Cee", Genre: "Hip-Hop", AddedAt: 400, Kind: "audio"},
	}
}

func TestGroupsAndSearch(t *testing.T) {
	a := Artists(songs())
	if a[0].Name != "Ana" || len(a[0].Tracks) != 2 {
		t.Fatalf("artists: %+v", a)
	}
	if got := Search(songs(), "ana piano"); len(got) != 1 || got[0].Key != "3" {
		t.Fatalf("search every word: %+v", got)
	}
	if al := Albums(songs()); al[0].Name != "Ana" && al[0].Name != "Calm" {
		t.Fatalf("albums fall back to artist: %+v", al)
	}
}

func TestMatchMusicMood(t *testing.T) {
	got, _, ok := MatchMusic(songs(), "play something chill")
	if !ok || got[0].Key != "1" {
		t.Fatalf("chill finds the lo-fi song first: %+v", got)
	}
	for _, g := range got {
		if g.Key == "2" || g.Key == "4" {
			t.Fatal("party / gym songs aren't chill")
		}
	}
	got, why, ok := MatchMusic(songs(), "play Gym Hype")
	if !ok || len(got) != 1 || got[0].Key != "4" || !strings.Contains(why, "gym hype") {
		t.Fatalf("a title wins: %+v %s", got, why)
	}
	if _, _, ok := MatchMusic(songs(), "zzzz"); ok {
		t.Fatal("nothing matches")
	}
}

func TestSmartListsAndMixes(t *testing.T) {
	now := int64(100) * day
	ev := []PlayEvent{{"1", now - day, 60000}, {"1", now - 2*day, 60000}, {"3", now - day, 40000}, {"3", now - day, 40000}, {"3", now - 3*day, 50000}}
	lists := SmartLists(songs(), ev, []string{"2", "gone"}, now)
	byID := map[string]SmartList{}
	for _, l := range lists {
		byID[l.ID] = l
	}
	if top := byID["smart:top"]; len(top.Keys) != 2 || top.Keys[0] != "3" {
		t.Fatalf("most played: %+v", top)
	}
	if !reflect.DeepEqual(byID["smart:liked"].Keys, []string{"2"}) {
		t.Fatal("liked keeps only songs in the library")
	}
	if nv := byID["smart:never"]; len(nv.Keys) != 2 {
		t.Fatalf("never played: %+v", nv)
	}
	mixes := DailyMixes(songs(), ev, 100, 25)
	if len(mixes) != 1 || mixes[0].Name != "Daily Mix 1" || !strings.HasPrefix(mixes[0].About, "Ana") || len(mixes[0].Keys) < 3 {
		t.Fatalf("daily mix from the top artist: %+v", mixes)
	}
	if !reflect.DeepEqual(DailyMixes(songs(), ev, 100, 25), mixes) {
		t.Fatal("same day, same mix")
	}
	w := MakeWrapped(ev, songs(), 0, now, "All time", func(int64) int { return 21 })
	if w == nil || w.Plays != 5 || w.TopSongs[0].Track.Key != "3" || w.TopArtists[0].Name != "Ana" || w.BusiestHour != 21 {
		t.Fatalf("wrapped: %+v", w)
	}
	if !strings.Contains(WrappedText(w), "21:00") || WrappedText(nil) == "" {
		t.Fatal("wrapped text")
	}
	dj := DJPicks(songs(), ev, map[string]bool{"2": true}, []string{"4"}, 3, 7)
	for _, d := range dj {
		if d.Key == "4" {
			t.Fatal("DJ never repeats a recent song")
		}
	}
	b, _ := json.Marshal(ev[:1])
	var back []PlayEvent
	_ = json.Unmarshal(b, &back)
	if string(b) != `[["1",8553600000,60000]]` || back[0] != ev[0] {
		t.Fatalf("play log json is compact: %s", b)
	}
}

func TestEQAndFades(t *testing.T) {
	l := BandLevels("Bass boost", EQBands, -1200, 1200)
	if l[0] != 600 || l[2] != 0 {
		t.Fatalf("bands: %v", l)
	}
	if mid := BandLevels("Bass boost", []int{120}, -1200, 1200)[0]; mid <= 350 || mid >= 600 {
		t.Fatalf("log interpolation: %d", mid)
	}
	if BandLevels("Rock", []int{60}, -300, 300)[0] != 300 {
		t.Fatal("clamped")
	}
	if FadeVolume(0, 300000, 6000) != 0.05 || FadeVolume(150000, 300000, 6000) != 1 || FadeVolume(0, 10000, 6000) != 1 {
		t.Fatal("fade curve")
	}
	if SleepVolume(15000, 30000) != 0.5 || SleepVolume(60000, 30000) != 1 {
		t.Fatal("sleep fade")
	}
}

func TestFixTagsAndImports(t *testing.T) {
	a, ti := FixTags("Burna Boy - Last Last (Official Music Video)", "BurnaBoyVEVO")
	if a != "Burna Boy" || ti != "Last Last" {
		t.Fatalf("fix tags: %q %q", a, ti)
	}
	a, ti = FixTags("Calm Down | Rema", "Rema - Topic")
	if a != "Rema" || ti != "Calm Down" {
		t.Fatalf("title then artist: %q %q", a, ti)
	}
	if ItunesArt(`{"results":[{"artworkUrl100":"https://x/100x100bb.jpg"}]}`) != "https://x/600x600bb.jpg" {
		t.Fatal("itunes art")
	}
	if YoutubePlaylistID("https://www.youtube.com/watch?v=abc&list=PLxyz1234567") != "PLxyz1234567" ||
		SpotifyPlaylistID("https://open.spotify.com/playlist/37i9dQZF1DXcBWIGoYBM5M?si=1") != "37i9dQZF1DXcBWIGoYBM5M" ||
		SpotifyAlbumID("spotify:album:4aawyAB9vmqN3uQ7FjRGTy") != "4aawyAB9vmqN3uQ7FjRGTy" {
		t.Fatal("playlist ids")
	}
	html := `<script id="__NEXT_DATA__" type="application/json">{"props":{"pageProps":{"state":{"data":{"entity":{"name":"Hits","trackList":[{"title":"One","subtitle":"Ana\u00a0B"},{"title":"Two","subtitle":""}]}}}}}}</script>`
	names, title := SpotifyEmbed(html)
	if title != "Hits" || !reflect.DeepEqual(names, []string{"Ana B - One", "Two"}) {
		t.Fatalf("spotify embed: %v %s", names, title)
	}
	yt := `{"metadata":{"playlistMetadataRenderer":{"title":"Mix"}},"x":[{"playlistVideoRenderer":{"videoId":"aaaaaaaaaaa","title":{"runs":[{"text":"Song"}]},"shortBylineText":{"simpleText":"Ana"}}}],"c":{"continuationCommand":{"token":"NEXT"}}}`
	items, next, name := YtPlaylist(yt)
	if name != "Mix" || next != "NEXT" || len(items) != 1 || items[0][1] != "Ana - Song" {
		t.Fatalf("yt playlist: %v %s %s", items, next, name)
	}
}

func TestTaste(t *testing.T) {
	now := int64(1_700_000_000_000)
	var ev []TasteEvent
	for i, tt := range []string{"Amapiano Night", "Amapiano Sunday", "Lo-fi Rain", "Gospel Hour", "Amapiano Drive"} {
		ev = PushTaste(ev, TasteEvent{"audio", tt, "DJ Zed - Topic", 1, now - int64(i)*day})
	}
	if a := TopAuthors(ev, "audio", now, 3); len(a) != 1 || a[0] != "DJ Zed" {
		t.Fatalf("authors cleaned: %v", a)
	}
	if s := TopStyles(ev, "audio", now, 3); len(s) == 0 || s[0] != "amapiano" {
		t.Fatalf("styles: %v", s)
	}
	ideas := Ideas(ev, "audio", now, 12)
	if ideas[0] != "DJ Zed mix" || ideas[1] != "Amapiano music" || len(ideas) != 12 {
		t.Fatalf("ideas: %v", ideas)
	}
	if !strings.Contains(TasteSummary(ev, now), "DJ Zed") || TasteSummary(ev[:2], now) != "" {
		t.Fatal("summary needs 5 events")
	}
}

func TestLyrics(t *testing.T) {
	l := ParseLrc("[00:01.50]Hello\n[00:03.5][00:10.123]Again\nplain line")
	if len(l) != 3 || l[0].Ms != 1500 || l[1].Ms != 3500 || l[2].Ms != 10123 || l[2].Text != "Again" {
		t.Fatalf("lrc: %+v", l)
	}
	if LrcCurrent(l, 0) != -1 || LrcCurrent(l, 4000) != 1 || LrcCurrent(l, 99999) != 2 {
		t.Fatal("current line")
	}
	if CleanTitle("Rema - Calm Down (Official Video) [HD]", "Rema") != "Calm Down" {
		t.Fatal("clean title")
	}
	r := ParseLrclib(`{"syncedLyrics":"[00:02.00]Hi","plainLyrics":"Hi","instrumental":false}`)
	if len(r.Synced) != 1 || r.Plain != "Hi" || r.Empty() {
		t.Fatalf("lrclib: %+v", r)
	}
	if !(LyricsResult{}).Empty() {
		t.Fatal("empty")
	}
}

func TestSubtitles(t *testing.T) {
	srt := "1\n00:00:01,000 --> 00:00:02,500\n<i>Hello</i>\n\n2\n00:00:03,000 --> 00:00:04,000\nWorld\n"
	c := ParseSubs(srt)
	if len(c) != 2 || c[0].Text != "Hello" || c[0].EndMs != 2500 {
		t.Fatalf("srt: %+v", c)
	}
	vtt := "WEBVTT\n\n00:00:01.000 --> 00:00:03.000\nline one\n\n00:00:03.000 --> 00:00:05.000\nline one\nline two\n"
	c = ParseSubs(vtt)
	if len(c) != 2 || c[1].Text != "line two" {
		t.Fatalf("rolling auto-captions are deduped: %+v", c)
	}
	j3 := `{"events":[{"tStartMs":0,"dDurationMs":5000,"segs":[{"utf8":"a"}]},{"tStartMs":2000,"dDurationMs":3000,"segs":[{"utf8":"b"}]}]}`
	c = ParseSubs(j3)
	if len(c) != 2 || c[0].EndMs != 2000 {
		t.Fatalf("json3 clipped: %+v", c)
	}
	x := `<transcript><text start="1.5" dur="2">it&amp;#39;s</text></transcript>`
	if c = ParseSubs(x); len(c) != 1 || c[0].Text != "it's" || c[0].StartMs != 1500 {
		t.Fatalf("srv1: %+v", c)
	}
	back := ParseSubs(ToVtt([]Cue{{1000, 2000, "a < b"}}))
	if len(back) != 1 || back[0].Text != "a < b" {
		t.Fatalf("vtt round trip: %+v", back)
	}
	if ClockMs("1:02:03.4") != 3723400 || ClockMs("x") != -1 {
		t.Fatal("clock")
	}
	ts := []Subtitle{{ID: "a", Lang: "en", Auto: true}, {ID: "b", Lang: "en-GB"}, {ID: "c", Lang: "fr"}}
	if p := PickSub(ts, "en"); p == nil || p.ID != "b" {
		t.Fatal("manual beats auto")
	}
	if PickSub(ts, "off") != nil || PickSub(ts, "") != nil {
		t.Fatal("off means off")
	}
	if o := OrderSubs(ts, "fr"); o[0].ID != "c" {
		t.Fatal("viewer's language first")
	}
	if SubLabel(Subtitle{Lang: "en", Auto: true}) != "English (auto-generated)" || LangCode("French") != "fr" {
		t.Fatal("labels")
	}
	if SubWithFormat("https://x/api?v=1&fmt=srv3&lang=en", "json3") != "https://x/api?v=1&lang=en&fmt=json3" {
		t.Fatal(SubWithFormat("https://x/api?v=1&fmt=srv3&lang=en", "json3"))
	}
	player := map[string]any{"captions": map[string]any{"playerCaptionsTracklistRenderer": map[string]any{"captionTracks": []any{
		map[string]any{"baseUrl": "https://c/1", "languageCode": "en", "vssId": "a.en", "kind": "asr", "name": map[string]any{"simpleText": "English"}}}}}}
	if yt := YtTracks(player); len(yt) != 1 || !yt[0].Auto || yt[0].ID != "yt-a.en" {
		t.Fatalf("yt tracks: %+v", yt)
	}
	if DecodeText([]byte{0x93, 'h', 'i', 0x94}) != "\u201chi\u201d" {
		t.Fatal("cp1252")
	}
}

func TestVideoSmart(t *testing.T) {
	e := ParseEpisode("The.Office.S02E05.720p")
	if e == nil || e.Series != "The Office" || e.Season != 2 || e.Episode != 5 {
		t.Fatalf("episode: %+v", e)
	}
	if e := ParseEpisode("Lessons - Episode 3"); e == nil || e.Episode != 3 || e.Series != "Lessons" {
		t.Fatalf("single: %+v", e)
	}
	v := []Track{{Key: "b", Title: "Show S01E02"}, {Key: "a", Title: "Show S01E01"}, {Key: "f", Title: "A Film"}}
	s := Series(v)
	if len(s) != 1 || s[0].Tracks[0].Key != "a" {
		t.Fatalf("series: %+v", s)
	}
	if n := NextEpisode(v[1], v); n == nil || n.Key != "b" || NextEpisode(v[0], v) != nil {
		t.Fatal("next episode")
	}
	if LearnIntro(0, 5000, 90000) != 90000 || LearnIntro(90000, 5000, 80000) != 85000 || LearnIntro(0, 400000, 450000) != 0 {
		t.Fatal("learn intro")
	}
	if !ShowSkip(10000, 90000) || ShowSkip(1000, 90000) || ShowSkip(88000, 90000) {
		t.Fatal("show skip")
	}
	cues := []Cue{{0, 1000, "a"}, {1000, 2000, "a"}, {5000, 6000, "b"}, {70000, 71000, "later"}}
	if tr := Transcript(cues, 6000, 1000); tr != "[0:00] a\n[0:05] b\n" {
		t.Fatalf("transcript: %q", tr)
	}
	b := Batches([]Cue{{0, 1, "aaaa"}, {1, 2, "bbbb"}, {2, 3, "cccc"}}, 20)
	if len(b) != 2 {
		t.Fatalf("batches: %d", len(b))
	}
	out := ApplyTranslation(b[0], "1|uno\n2) dos")
	if out[0].Text != "uno" || out[1].Text != "dos" || TranslatedShare(b[0], out) != 1 {
		t.Fatalf("translation: %+v", out)
	}
	clear := ToClear([]Track{{Key: "x", Kind: "video", Source: "download", Title: "Show S01E01"}, {Key: "y", Kind: "video", Source: "download", Title: "Film"}},
		map[string]int64{"x": 1, "y": 1}, 100*day, day)
	if len(clear) != 1 || clear[0].Key != "x" {
		t.Fatal("smart downloads clear episodes only")
	}
	if fv := FindVideo([]Track{{Title: "Big Buck Bunny"}, {Title: "Big Buck Bunny (Extended Cut)"}}, "big buck bunny"); fv == nil || fv.Title != "Big Buck Bunny" {
		t.Fatal("find video")
	}
}

func TestPick(t *testing.T) {
	items := []Item{
		{ID: "1", Title: "Rema - Calm Down (Official Music Video)", Author: "RemaVEVO", DurationSec: 239},
		{ID: "2", Title: "Calm Down (Live)", Author: "Someone", DurationSec: 260},
		{ID: "3", Title: "Calm Down 1 hour loop", Author: "Loops", DurationSec: 3600},
	}
	r := PickRank("Rema - Calm Down", "audio", items)
	if r[0].Item.ID != "1" || !PickConfident(r) {
		t.Fatalf("rank: %+v", r)
	}
	if PickConfident(PickRank("something else", "audio", items)) {
		t.Fatal("not confident")
	}
	ch := []Choice{{ID: "a", Kbps: 128, Ext: "m4a"}, {ID: "b", Kbps: 48, Ext: "m4a"}, {ID: "o", Kbps: 160, Ext: "opus"}}
	if PickChoice("audio", ch, "high", "").ID != "a" || PickChoice("audio", ch, "small", "").ID != "b" || PickChoice("audio", ch, "", "opus").ID != "o" {
		t.Fatal("audio choice")
	}
	vc := []Choice{{ID: "1080", Height: 1080}, {ID: "720", Height: 720}, {ID: "360", Height: 360}}
	if PickChoice("video", vc, "", "").ID != "720" || PickChoice("video", vc, "best", "").ID != "1080" || PickChoice("video", vc, "480p", "").ID != "360" {
		t.Fatal("video choice")
	}
}

func TestDownloadLogic(t *testing.T) {
	j := DlJob{ID: "d", Status: DlRunning, Parts: []DlPart{{Total: 100}, {Total: 300}}}
	j = DlOnProgress(j, 1, 150, -1)
	if j.Progress() != 0.375 || j.Attempts != 0 {
		t.Fatalf("progress: %v", j.Progress())
	}
	e := DlOnError(j, "net", true, 1000)
	if e.Status != DlQueued || e.NextTryAt != 3000 || e.Attempts != 1 {
		t.Fatalf("transient retries: %+v", e)
	}
	if DlOnError(j, "space", false, 0).Status != DlFailed {
		t.Fatal("fatal fails")
	}
	j.Attempts = DlMaxAttempts - 1
	if DlOnError(j, "net", true, 0).Status != DlFailed {
		t.Fatal("attempt cap")
	}
	if Backoff(0) != 2000 || Backoff(30) != 300000 {
		t.Fatal("backoff")
	}
	if DlRange(0, 100, 8<<20) != "bytes=0-99" || DlRange(10, 0, 10) != "bytes=10-19" {
		t.Fatal("range")
	}
	if ExpiryOf("https://r1.googlevideo.com/videoplayback?expire=1700000000&x=1") != 1700000000 || !URLExpired(1700000000, 1700000000*1000-60000) {
		t.Fatal("expiry")
	}
	if TotalFromContentRange("bytes 0-99/5000") != 5000 || TotalFromContentRange("") != -1 {
		t.Fatal("content-range")
	}
	jobs := []DlJob{{ID: "a", Status: DlDone, CreatedAt: 1}, {ID: "b", Status: DlQueued, CreatedAt: 3}, {ID: "c", Status: DlRunning, CreatedAt: 2}, {ID: "d", Status: DlQueued, CreatedAt: 0, NextTryAt: 500}}
	if n := DlNext(jobs, 100); n == nil || n.ID != "c" || DlNextWake(jobs, 100) != 500 {
		t.Fatal("next job")
	}
	if p := DlPause(DlJob{Status: DlRunning}); p.Status != DlPaused || DlResume(p).Status != DlQueued {
		t.Fatal("pause/resume")
	}
	if SafeFileName(`AC/DC: Back in Black`, "mp3") != "AC-DC- Back in Black.mp3" || SafeFileName("...", "mp3") != "Blaze.mp3" {
		t.Fatal(SafeFileName(`AC/DC: Back in Black`, "mp3"))
	}
	if onStatus(206) != rsAppend || onStatus(200) != rsRestart || onStatus(416) != rsComplete || onStatus(403) != rsRetry || onStatus(404) != rsFail {
		t.Fatal("statuses")
	}
}

func TestStoreParsing(t *testing.T) {
	if LicenceLabel("https://creativecommons.org/licenses/by-sa/4.0/") != "CC BY-SA 4.0" || LicenceLabel("http://creativecommons.org/publicdomain/zero/1.0/") != "CC0 (public domain)" {
		t.Fatal("licence")
	}
	if !LicenceFree("https://creativecommons.org/licenses/by/3.0") || LicenceFree("https://example.com") {
		t.Fatal("free")
	}
	if ParseClock("1:02:05") != 3725 || ParseClock("a:b") != -1 || ClockText(65000) != "1:05" || SizeText(4_100_000) != "4.1 MB" {
		t.Fatal("text")
	}
	as := `{"response":{"numFound":40,"docs":[{"identifier":"x1","title":"T","creator":["A"],"licenseurl":"https://creativecommons.org/licenses/by/4.0/","runtime":"3:12","downloads":12345},{"identifier":"x2","licenseurl":"https://example.com"}]}}`
	p, err := ArchiveSearch(as, "archive-audio", 1, 30, true, "", true)
	if err != nil || len(p.Items) != 1 || p.Items[0].Author != "A" || p.Items[0].DurationSec != 192 || p.Next != "2" || p.Items[0].Views != "12,345 downloads" {
		t.Fatalf("archive search: %+v %v", p, err)
	}
	md := `{"metadata":{"identifier":"x1","title":"Album"},"files":[{"name":"01 Song.mp3","format":"VBR MP3","size":"3000000","length":"120"},{"name":"01 Song.ogg","format":"Ogg Vorbis","size":"2000000"},{"name":"cover.png","format":"PNG"}]}`
	d, _ := ArchiveDetail(md, "audio", "")
	ch := ArchiveAudioChoices(d)
	if len(ch) != 2 || ch[0].Ext != "mp3" || ch[0].Kbps != 200 {
		t.Fatalf("archive choices: %+v", ch)
	}
	if !strings.HasSuffix(d.Formats[0].URL, "/01%20Song.mp3") {
		t.Fatal(d.Formats[0].URL)
	}
	vd, _ := ArchiveDetail(`{"metadata":{"identifier":"f"},"files":[{"name":"a.mp4","format":"h.264","height":"1080","size":"9"},{"name":"b.mp4","format":"512Kb MPEG4","height":"480","size":"5"}]}`, "video", "Public domain")
	if dv := ArchiveDefaultVideo(ArchiveVideoChoices(vd)); dv == nil || dv.Height != 480 || vd.Licence != "Public domain" {
		t.Fatalf("default film: %+v", dv)
	}
	js := `{"headers":{"status":"success","results_count":30},"results":[{"id":"7","name":"Song","artist_name":"Ana","duration":200,"audio":"https://s/a?format=mp31","audiodownload":"https://s/d","license_ccurl":"http://creativecommons.org/licenses/by-nc/3.0/"}]}`
	jp, _ := JamendoSearch(js, 0, 30)
	if len(jp.Items) != 1 || jp.Next != "30" || jp.Items[0].Licence != "CC BY-NC 3.0" {
		t.Fatalf("jamendo: %+v", jp)
	}
	jd := JamendoDetail(jp.Items[0])
	if len(jd.Formats) != 2 || jd.Formats[0].Key != "mp32" {
		t.Fatal("jamendo detail")
	}
	if _, err := JamendoSearch(`{"headers":{"status":"failed","code":5,"error_message":"bad client id"}}`, 0, 30); err == nil || !strings.Contains(err.Error(), "client_id") {
		t.Fatal("jamendo error")
	}
	if JamendoWithFormat("https://x?a=1&format=mp32", "mp31") != "https://x?a=1&format=mp31" || JamendoWithFormat("https://x", "mp31") != "https://x?format=mp31" {
		t.Fatal("with format")
	}
}

func TestYouTubeParsing(t *testing.T) {
	for in, want := range map[string]string{
		"https://youtu.be/dQw4w9WgXcQ?t=3": "dQw4w9WgXcQ", "https://www.youtube.com/watch?v=dQw4w9WgXcQ&list=x": "dQw4w9WgXcQ",
		"https://m.youtube.com/shorts/dQw4w9WgXcQ": "dQw4w9WgXcQ", "https://music.youtube.com/watch?v=dQw4w9WgXcQ": "dQw4w9WgXcQ",
		"https://example.com/watch?v=dQw4w9WgXcQ": "",
	} {
		if YoutubeID(in) != want {
			t.Errorf("%s → %s", in, YoutubeID(in))
		}
	}
	if FirstURL("look: youtube.com/watch?v=dQw4w9WgXcQ.") != "https://youtube.com/watch?v=dQw4w9WgXcQ" {
		t.Fatal(FirstURL("look: youtube.com/watch?v=dQw4w9WgXcQ."))
	}
	if ExtOf("https://x/a/song.MP3?x=1") != "mp3" || ExtOf("https://x/page") != "" || FileTitle("https://x/My_Great-Song.mp3") != "My Great Song" {
		t.Fatal("ext / title")
	}
	if MediaInPage(`<meta property="og:video" content="/v/clip.mp4">`, "https://site.com/page") != "https://site.com/v/clip.mp4" {
		t.Fatal("og video")
	}
	search := `{"contents":[{"videoRenderer":{"videoId":"aaaaaaaaaaa","title":{"runs":[{"text":"Song"}]},"ownerText":{"runs":[{"text":"Ana"}]},"lengthText":{"simpleText":"3:21"}}},{"videoRenderer":{"videoId":"bbbbbbbbbbb","title":{"runs":[{"text":"Live"}]}}},{"continuationCommand":{"token":"T"}}]}`
	p := YtSearchParse(search, "youtube")
	if len(p.Items) != 1 || p.Items[0].DurationSec != 201 || p.Next != "T" {
		t.Fatalf("search: %+v", p)
	}
	player := `{"playabilityStatus":{"status":"OK"},"videoDetails":{"title":"V","author":"Ch","lengthSeconds":"100"},"streamingData":{
	 "formats":[{"itag":18,"url":"https://g/18","mimeType":"video/mp4; codecs=\"avc1.42001E, mp4a.40.2\"","height":360,"qualityLabel":"360p","bitrate":500000}],
	 "adaptiveFormats":[{"itag":137,"url":"https://g/137","mimeType":"video/mp4; codecs=\"avc1.640028\"","height":1080,"bitrate":4000000,"contentLength":"50000000"},
	   {"itag":140,"url":"https://g/140","mimeType":"audio/mp4; codecs=\"mp4a.40.2\"","bitrate":130000,"contentLength":"1600000"},
	   {"itag":251,"url":"https://g/251","mimeType":"audio/webm; codecs=\"opus\"","bitrate":160000},
	   {"itag":999,"signatureCipher":"s=x","mimeType":"audio/mp4"}]}}`
	d, err := YtDetailParse(player, "vid")
	if err != nil || len(d.Formats) != 4 || d.ID != "yt:vid" {
		t.Fatalf("player: %+v %v", d, err)
	}
	a := YtChoices("audio", d, false)
	if len(a) != 2 || a[0].Ext != "m4a" || a[1].Ext != "opus" {
		t.Fatalf("audio choices: %+v", a)
	}
	if v := YtChoices("video", d, true); len(v) != 2 || v[0].Height != 1080 || len(v[0].Keys) != 2 {
		t.Fatalf("merged video choices: %+v", v)
	}
	if v := YtChoices("video", d, false); len(v) != 1 || v[0].Height != 360 {
		t.Fatalf("without ffmpeg only one-file video: %+v", v)
	}
	if s := YtStreamPick("video", d); len(s) != 1 || s[0].Key != "18" {
		t.Fatal("stream pick")
	}
	if _, err := YtDetailParse(`{"playabilityStatus":{"status":"LOGIN_REQUIRED","reason":"Sign in to confirm your age"}}`, "x"); err == nil || !strings.Contains(err.Error(), "age") {
		t.Fatal("unplayable")
	}
	if ProbeAt(1_000_000) != 0 || ProbeAt(0) != 1_500_000 {
		t.Fatal("probe")
	}
}

// writeMP3 writes a tiny file with an ID3v2.3 tag (title, artist, genre).
func writeMP3(t *testing.T, path, title, artist, genre string) {
	frame := func(id, text string) []byte {
		body := append([]byte{3}, []byte(text)...) // UTF-8
		n := len(body)
		return append([]byte{id[0], id[1], id[2], id[3], byte(n >> 24), byte(n >> 16), byte(n >> 8), byte(n), 0, 0}, body...)
	}
	var frames []byte
	frames = append(frames, frame("TIT2", title)...)
	frames = append(frames, frame("TPE1", artist)...)
	frames = append(frames, frame("TCON", genre)...)
	n := len(frames)
	hdr := []byte{'I', 'D', '3', 3, 0, 0, byte(n >> 21 & 0x7f), byte(n >> 14 & 0x7f), byte(n >> 7 & 0x7f), byte(n & 0x7f)}
	data := append(append(hdr, frames...), make([]byte, 4096)...)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func newTestMedia(t *testing.T) (*Media, string, *[]Cmd) {
	home := t.TempDir()
	m := New(filepath.Join(home, ".blaze", "media"))
	m.HomeDir = home
	var cmds []Cmd
	m.Emit = func(ev string, data any) {
		if c, ok := data.(Cmd); ok && ev == "media:cmd" {
			cmds = append(cmds, c)
		}
	}
	return m, home, &cmds
}

func TestScanServeAndLibrary(t *testing.T) {
	m, home, cmds := newTestMedia(t)
	writeMP3(t, filepath.Join(home, "Music", "Ana", "01 - slow.mp3"), "Slow Morning", "Ana", "Lo-Fi")
	writeMP3(t, filepath.Join(home, "Music", "party.mp3"), "", "", "Dance")
	if err := os.WriteFile(filepath.Join(home, "Music", "notes.txt"), make([]byte, 5000), 0o644); err != nil {
		t.Fatal(err)
	}
	vid := filepath.Join(home, "Videos", "Show S01E01.mp4")
	_ = os.MkdirAll(filepath.Dir(vid), 0o755)
	_ = os.WriteFile(vid, make([]byte, 20000), 0o644)
	_ = os.WriteFile(filepath.Join(home, "Videos", "Show S01E01.en.srt"), []byte("1\n00:00:01,000 --> 00:00:02,000\nHi\n"), 0o644)
	_ = os.MkdirAll(filepath.Join(home, "Music", ".hidden"), 0o755)
	writeMP3(t, filepath.Join(home, "Music", ".hidden", "x.mp3"), "Hidden", "", "")

	got := m.ScanNow()
	m.scanned = got
	m.writeJSON("scan.json", got)
	if len(got) != 3 {
		t.Fatalf("scanned %d: %+v", len(got), got)
	}
	songs := m.Songs()
	var slow, party Track
	for _, s := range songs {
		switch {
		case s.Artist == "Ana":
			slow = s
		case strings.HasSuffix(s.Path, "party.mp3"):
			party = s
		}
	}
	if slow.Title != "Slow Morning" || slow.Genre != "Lo-Fi" || party.Title != "party" {
		t.Fatalf("tags and file-name titles: %+v / %+v", slow, party)
	}
	if again := m.ScanNow(); len(again) != 3 || again[0].Key != got[0].Key {
		t.Fatal("rescan from cache")
	}
	// Handler: file with Range, token check, subtitles, unknown key.
	srv := httptest.NewServer(m.Handler("tok"))
	defer srv.Close()
	req, _ := http.NewRequest("GET", srv.URL+"/media/file?t=tok&k="+urlq(slow.Key), nil)
	req.Header.Set("Range", "bytes=0-9")
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != 206 || resp.Header.Get("Content-Type") != "audio/mpeg" || resp.Header.Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("range: %v %v", resp, err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(b[:3]) != "ID3" || len(b) != 10 {
		t.Fatalf("bytes: %q", b)
	}
	if r, _ := http.Get(srv.URL + "/media/file?k=" + urlq(slow.Key)); r.StatusCode != 403 {
		t.Fatal("token required")
	}
	if r, _ := http.Get(srv.URL + "/media/file?t=tok&k=" + urlq("file:/etc/passwd")); r.StatusCode != 404 {
		t.Fatal("only library files")
	}
	vk := FileKey(vid)
	if subs := m.VideoSubs(vk, false); len(subs) != 1 || subs[0].Lang != "en" {
		t.Fatalf("sidecar subs: %+v", subs)
	}
	r, _ := http.Get(srv.URL + "/media/sub?t=tok&v=" + urlq(vk) + "&id=" + m.VideoSubs(vk, false)[0].ID)
	body, _ := io.ReadAll(r.Body)
	if !strings.HasPrefix(string(body), "WEBVTT") || !strings.Contains(string(body), "Hi") {
		t.Fatalf("sub as vtt: %s", body)
	}
	// Stream proxy passes Range and the user-agent.
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != "UA1" {
			w.WriteHeader(400)
			return
		}
		w.Header().Set("Content-Range", "bytes 0-1/2")
		w.WriteHeader(206)
		_, _ = w.Write([]byte("ok"))
	}))
	defer up.Close()
	path := m.AddStream(up.URL, "UA1", "audio/mpeg")
	pr, _ := http.NewRequest("GET", srv.URL+path+"?t=tok", nil)
	pr.Header.Set("Range", "bytes=0-1")
	presp, _ := http.DefaultClient.Do(pr)
	pb, _ := io.ReadAll(presp.Body)
	if presp.StatusCode != 206 || string(pb) != "ok" {
		t.Fatalf("proxy: %d %s", presp.StatusCode, pb)
	}
	// Library state.
	if !m.ToggleLike(slow.Key) || len(m.Library().Liked) != 1 {
		t.Fatal("like")
	}
	m.Played(slow.Key)
	m.LogPlay(slow.Key, time.Now().UnixMilli(), 60000)
	m.LogPlay("stream:x", time.Now().UnixMilli(), 60000)
	if lib := m.Library(); len(lib.Recent) != 1 || len(m.plays) != 1 {
		t.Fatal("recents and play log")
	}
	m.SavePosition(vk, 30000, 600000)
	if m.Position(vk) != 30000 {
		t.Fatal("position")
	}
	m.SavePosition(vk, 599000, 600000)
	if m.Position(vk) != 0 || m.Prefs().Finished[vk] == 0 {
		t.Fatal("finished videos start over and are marked watched")
	}
	pls := m.EditPlaylists(func(l []Playlist, now int64) []Playlist {
		return PlAdd(PlCreate(l, "Mine", "p1", now), "p1", []string{slow.Key}, now)
	})
	if len(pls) != 1 || len(pls[0].Keys) != 1 {
		t.Fatal("playlist")
	}
	if !m.SetTags(party.Key, "Party Song", "Ben") {
		t.Fatal("set tags")
	}
	if p, _ := m.Find(party.Key); p.Title != "Party Song" || p.Artist != "Ben" {
		t.Fatalf("tags override: %+v", p)
	}
	// Reload from disk keeps everything.
	m2 := New(m.Dir())
	m2.HomeDir = home
	if lib := m2.Library(); len(lib.Liked) != 1 || len(lib.Playlists) != 1 || len(lib.Songs) != 2 || len(lib.Videos) != 1 {
		t.Fatalf("persisted: %+v", lib)
	}
	// Agent ops.
	out := m.MusicPlay(map[string]any{"action": "play", "query": "something chill"})
	if !strings.Contains(out, "Slow Morning") || len(*cmds) != 1 || (*cmds)[0].Op != "play" || (*cmds)[0].Keys[0] != slow.Key || !(*cmds)[0].Shuffle {
		t.Fatalf("play something chill: %s %+v", out, *cmds)
	}
	if out := m.MusicPlay(map[string]any{"action": "pause"}); !strings.Contains(out, "Nothing is playing") {
		t.Fatalf("pause with nothing reported: %s", out)
	}
	m.SetPlayer(PlayerState{Kind: "audio", Key: slow.Key, Title: "Slow Morning", Artist: "Ana", Playing: true, PosMs: 61000, DurMs: 180000, Count: 3, Index: 0, UpNext: []string{"Party Song"}})
	if out := m.MusicPlay(map[string]any{"action": "now"}); !strings.Contains(out, "Playing: \u201cSlow Morning\u201d by Ana (1:01 of 3:00, 1 of 3") {
		t.Fatalf("now: %s", out)
	}
	if out := m.MusicPlay(map[string]any{"action": "next"}); out != "Skipped to the next song." || (*cmds)[len(*cmds)-1].Op != "next" {
		t.Fatal("next")
	}
	if out := m.MusicPlay(map[string]any{"action": "volume", "level": float64(40)}); out != "Volume 40%." || (*cmds)[len(*cmds)-1].Num != 0.4 {
		t.Fatal(out)
	}
	if out := m.MusicPlay(map[string]any{"action": "play", "playlist": "mine"}); !strings.Contains(out, "Mine (1 songs)") {
		t.Fatal(out)
	}
	if out := m.MusicOps(map[string]any{"action": "eq", "preset": "bass boost"}); out != "Equalizer set to Bass boost." || m.Prefs().EQ != "Bass boost" {
		t.Fatal(out)
	}
	if out := m.MusicOps(map[string]any{"action": "wrapped", "period": "all"}); !strings.Contains(out, "All time") {
		t.Fatal(out)
	}
	if out := m.VideoOps(map[string]any{"action": "play", "query": "show"}); !strings.Contains(out, "Show S01E01") || (*cmds)[len(*cmds)-1].Op != "video" {
		t.Fatal(out)
	}
	if out := m.VideoOps(map[string]any{"action": "recap"}); !strings.Contains(out, "isn't playing") {
		t.Fatal(out)
	}
	m.SetPlayer(PlayerState{Kind: "video", Key: vk, PosMs: 5000})
	m.Ask = func(p string) (string, error) {
		if !strings.Contains(p, "[0:01] Hi") {
			t.Fatalf("recap prompt: %s", p)
		}
		return "- they said hi", nil
	}
	if out := m.VideoOps(map[string]any{"action": "recap"}); out != "- they said hi" {
		t.Fatal(out)
	}
	m.Ask = func(p string) (string, error) { return "1|Salut", nil }
	o, msg := m.TranslateSubs(vk, "French", "", nil)
	if o.ID != "ai-french" || o.Lang != "fr" || !strings.Contains(msg, "French") || len(m.SavedSubs(vk)) != 1 {
		t.Fatalf("translate: %+v %s", o, msg)
	}
	if out := m.VideoOps(map[string]any{"action": "subtitles", "language": "French"}); !strings.Contains(out, "French (AI)") || (*cmds)[len(*cmds)-1].Value != "ai-french" {
		t.Fatal(out)
	}
	// Deleting: folder files need the user's yes.
	if err := m.Delete(party.Key, false); err == nil {
		t.Fatal("folder files aren't deleted without confirmation")
	}
	if err := m.Delete(party.Key, true); err != nil || len(m.Songs()) != 1 {
		t.Fatal("delete")
	}
	if len(m.Sources("audio")) != 3 || len(m.Sources("video")) != 3 {
		t.Fatalf("sources: %+v %+v", m.Sources("audio"), m.Sources("video"))
	}
}

func urlq(s string) string {
	return strings.NewReplacer("%", "%25", "/", "%2F", ":", "%3A", " ", "%20", "&", "%26", "+", "%2B").Replace(s)
}

func TestDownloaderEndToEnd(t *testing.T) {
	m, home, _ := newTestMedia(t)
	data := make([]byte, 300000)
	for i := range data {
		data[i] = byte(i)
	}
	hits := 0
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		http.ServeContent(w, r, "a.mp3", time.Now(), strings.NewReader(string(data)))
	}))
	defer up.Close()
	it := Item{Source: "jamendo", ID: "7", Title: "Song / One", Author: "Ana"}
	d := Detail{ID: "7", Title: "Song / One", Author: "Ana", LengthSec: 10, Formats: []Format{{Key: "mp32", URL: up.URL + "/a.mp3", Mime: "audio/mpeg", ContentLength: int64(len(data))}}}
	ch := Choice{ID: "mp32", Label: "High", Keys: []string{"mp32"}, Ext: "mp3"}
	job := m.AddDownloadJob(it, d, ch, "audio")
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if j, _ := m.dl.get(job.ID); j.Status == DlDone || j.Status == DlFailed {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	j, _ := m.dl.get(job.ID)
	if j.Status != DlDone {
		t.Fatalf("download: %+v", j)
	}
	want := filepath.Join(home, "Music", "Blaze", "Music", "Song - One.mp3")
	b, err := os.ReadFile(want)
	if err != nil || len(b) != len(data) || j.Out != want {
		t.Fatalf("saved file: %v %d", err, len(b))
	}
	songs := m.Songs()
	if len(songs) != 1 || songs[0].Source != "download" || songs[0].Artist != "Ana" {
		t.Fatalf("library: %+v", songs)
	}
	m.DismissDownload(job.ID)
	if len(m.Downloads()) != 0 || len(m.Songs()) != 1 {
		t.Fatal("dismiss keeps the file")
	}
	m.Close()
}
