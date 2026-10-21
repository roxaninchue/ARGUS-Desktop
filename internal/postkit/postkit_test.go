package postkit

import (
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// The validator must say exactly what the published schema says.
func TestValidatorFollowsSchema(t *testing.T) {
	var s struct {
		ID         string                     `json:"$id"`
		Required   []string                   `json:"required"`
		Properties map[string]json.RawMessage `json:"properties"`
		Rules      struct {
			Limits map[string]int `json:"limits"`
		} `json:"x-blaze-rules"`
	}
	if err := json.Unmarshal(SchemaJSON, &s); err != nil {
		t.Fatal(err)
	}
	if s.ID != "https://blazeinchue.pages.dev/schemas/postkit-1.json" {
		t.Fatalf("schema id %q", s.ID)
	}
	want := []string{"schema", "language", "hook", "hashtags", "alt_text", "platforms", "crops"}
	if !reflect.DeepEqual(s.Required, want) {
		t.Fatalf("required %v", s.Required)
	}
	var props struct {
		Schema   struct{ Const string } `json:"schema"`
		Language struct{ Pattern string }
		Idea     struct{ MaxLength int }
		Hook     struct{ MinLength, MaxLength int }
		Hashtags struct {
			MinItems, MaxItems int
			Items              struct{ Pattern string }
		}
		AltText   struct{ MaxLength int } `json:"alt_text"`
		Platforms struct {
			Properties map[string]any
		}
		Crops struct {
			Items struct {
				Properties struct {
					Ratio struct{ Enum []string }
				}
			}
		}
	}
	b, _ := json.Marshal(s.Properties)
	if err := json.Unmarshal(b, &props); err != nil {
		t.Fatal(err)
	}
	if props.Schema.Const != SchemaID || props.Language.Pattern != langRe.String() || props.Idea.MaxLength != MaxIdea ||
		props.Hook.MaxLength != 120 || props.AltText.MaxLength != 125 || props.Hashtags.MinItems != 3 || props.Hashtags.MaxItems != 3 ||
		props.Hashtags.Items.Pattern != tagRe.String() {
		t.Fatalf("scalar rules drifted: %+v", props)
	}
	if !reflect.DeepEqual(props.Crops.Items.Properties.Ratio.Enum, Ratios) {
		t.Fatalf("ratios %v", props.Crops.Items.Properties.Ratio.Enum)
	}
	var ids []string
	for id := range props.Platforms.Properties {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var ours []string
	for _, p := range Platforms {
		ours = append(ours, p.ID)
		lim := s.Rules.Limits[p.ID]
		if p.ID == "reddit" {
			lim = s.Rules.Limits["reddit_body"]
			if s.Rules.Limits["reddit_title"] != p.Title {
				t.Fatalf("reddit title limit")
			}
		}
		if lim != p.Limit {
			t.Fatalf("%s limit %d, schema says %d", p.ID, p.Limit, lim)
		}
	}
	sort.Strings(ours)
	if !reflect.DeepEqual(ids, ours) {
		t.Fatalf("platforms %v vs %v", ids, ours)
	}
}

func goodKit() *Kit {
	return &Kit{Schema: SchemaID, Language: "en", Idea: "Sunset run, see https://blaze.test/run",
		Hook: "The sunset made me run faster", Hashtags: []string{"#sunset", "#run", "#Lusaka"}, AltText: "A runner on a red dirt road at sunset",
		Platforms: map[string]Post{"x": {Caption: "The sunset made me run faster\n\nhttps://blaze.test/run\n\n#sunset #run #Lusaka"}},
		Crops:     Crops(1200, 800, &Focus{X: 0.5, Y: 0.5}), Source: &Source{Width: 1200, Height: 800, Focus: &Focus{X: 0.5, Y: 0.5}}}
}

func TestValidate(t *testing.T) {
	if p := Validate(goodKit()); len(p) > 0 {
		t.Fatalf("good kit: %v", p)
	}
	bad := map[string]func(k *Kit){
		"schema":           func(k *Kit) { k.Schema = "blaze.postkit/2" },
		"language":         func(k *Kit) { k.Language = "english!" },
		"hook empty":       func(k *Kit) { k.Hook = "" },
		"hook 13 words":    func(k *Kit) { k.Hook = "one two three four five six seven eight nine ten eleven twelve thirteen" },
		"hook hashtag":     func(k *Kit) { k.Hook = "Run with me #sunset" },
		"two hashtags":     func(k *Kit) { k.Hashtags = k.Hashtags[:2] },
		"hashtag no #":     func(k *Kit) { k.Hashtags[0] = "sunset" },
		"hashtag punct":    func(k *Kit) { k.Hashtags[0] = "#sun-set" },
		"hashtag twice":    func(k *Kit) { k.Hashtags[1] = k.Hashtags[0] },
		"alt too long":     func(k *Kit) { k.AltText = strings.Repeat("a", 126) },
		"no platforms":     func(k *Kit) { k.Platforms = map[string]Post{} },
		"unknown platform": func(k *Kit) { k.Platforms["myspace"] = Post{Caption: "hi"} },
		"x over 280": func(k *Kit) {
			k.Platforms["x"] = Post{Caption: strings.Repeat("word ", 56) + "https://blaze.test/run"}
		},
		"dropped link": func(k *Kit) { k.Platforms["x"] = Post{Caption: "The sunset made me run faster"} },
		"no crops":     func(k *Kit) { k.Crops = nil },
		"bad ratio":    func(k *Kit) { k.Crops[0].Ratio = "3:2" },
		"wrong shape":  func(k *Kit) { k.Crops[0].W += 50 },
		"outside":      func(k *Kit) { k.Crops[0].X = 1000 },
		"focus":        func(k *Kit) { k.Source.Focus.X = 1.5 },
		"shorts title": func(k *Kit) {
			k.Platforms["youtube_shorts"] = Post{Caption: strings.Repeat("a", 101) + " https://blaze.test/run"}
		},
	}
	for name, mut := range bad {
		k := goodKit()
		mut(k)
		if p := Validate(k); len(p) == 0 {
			t.Errorf("%s: not caught", name)
		}
	}
	// X counts a long link as 23: 250 letters + a 60-character link is 274, valid.
	k := goodKit()
	k.Idea = ""
	k.Platforms["x"] = Post{Caption: strings.Repeat("a", 250) + " https://blaze.test/" + strings.Repeat("p", 40)}
	if p := Validate(k); len(p) > 0 {
		t.Fatalf("X link weighting: %v", p)
	}
}

func TestParseStrict(t *testing.T) {
	b, _ := json.Marshal(goodKit())
	if k, p := Parse(b); k == nil || len(p) > 0 {
		t.Fatalf("round trip: %v", p)
	}
	var m map[string]any
	json.Unmarshal(b, &m)
	m["extra"] = 1
	b2, _ := json.Marshal(m)
	if _, p := Parse(b2); len(p) == 0 {
		t.Fatal("unknown field accepted")
	}
	delete(m, "extra")
	delete(m, "alt_text")
	b3, _ := json.Marshal(m)
	if _, p := Parse(b3); len(p) == 0 || !strings.Contains(strings.Join(p, ";"), "alt_text is missing") {
		t.Fatalf("missing alt_text: %v", p)
	}
	if _, p := Parse([]byte(`{"schema":"blaze.postkit/1","language":"en","hook":"h","hashtags":["#a","#b","#c"],"alt_text":"a","platforms":{"x":{"text":"hi"}},"crops":[{"ratio":"1:1","x":0,"y":0,"w":1,"h":1}]}`)); len(p) == 0 {
		t.Fatal("post without caption accepted")
	}
}

func TestTrim(t *testing.T) {
	s := "First sentence is here. Second one is longer and goes on and on and on."
	if got := Trim(s, 40, Len); got != "First sentence is here." {
		t.Fatalf("sentence cut: %q", got)
	}
	if got := Trim("alpha beta gamma delta", 13, Len); got != "alpha beta" {
		t.Fatalf("word cut: %q", got)
	}
	// Never inside a link: the cut falls before it.
	if got := Trim("see https://blaze.test/a-long-path now", 20, Len); got != "see" {
		t.Fatalf("token cut: %q", got)
	}
	if got := Trim("ok", 10, Len); got != "ok" {
		t.Fatal("short text changed")
	}
	if got := Trim("ñandú ñandú ñandú", 11, Len); Len(got) > 11 || got != "ñandú ñandú" {
		t.Fatalf("code points: %q", got)
	}
	if XLen("go https://blaze.test/"+strings.Repeat("x", 100)) != 26 {
		t.Fatal("XLen")
	}
}

func TestTokens(t *testing.T) {
	got := Tokens("Ping @blaze_app or mail hi@blaze.test, see https://blaze.test/x. Use {name} and {{city}} with `blaze up`.")
	want := []string{"@blaze_app", "hi@blaze.test", "https://blaze.test/x", "{name}", "{{city}}", "`blaze up`"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%q", got)
	}
}

func TestCrops(t *testing.T) {
	for _, sz := range [][2]int{{1200, 800}, {800, 1200}, {1000, 1000}, {4032, 3024}} {
		for _, f := range []*Focus{nil, {X: 0, Y: 0}, {X: 1, Y: 1}, {X: 0.3, Y: 0.7}} {
			cs := Crops(sz[0], sz[1], f)
			if len(cs) != 4 {
				t.Fatal("4 crops")
			}
			k := goodKit()
			k.Crops, k.Source = cs, &Source{Width: sz[0], Height: sz[1], Focus: f}
			if p := Validate(k); len(p) > 0 {
				t.Fatalf("%v %v: %v", sz, f, p)
			}
		}
	}
	// Centred on the subject where the picture allows it.
	c := Crops(2000, 1000, &Focus{X: 0.25, Y: 0.5})[0] // 1:1 is 1000x1000
	if c.W != 1000 || c.X != 0 {
		t.Fatalf("clamped crop %+v", c)
	}
	c = Crops(3000, 1000, &Focus{X: 0.5, Y: 0.5})[0]
	if c.X != 1000 {
		t.Fatalf("centred crop %+v", c)
	}
}

// ---------------- end to end with a stubbed model ----------------

type stub struct {
	replies []string
	calls   []call
	err     error
}
type call struct{ system, user, photo string }

func (s *stub) ask(_ context.Context, system, user, photo string) (string, error) {
	s.calls = append(s.calls, call{system, user, photo})
	if s.err != nil {
		return "", s.err
	}
	r := s.replies[0]
	if len(s.replies) > 1 {
		s.replies = s.replies[1:]
	}
	return r, nil
}

const ideaReply = "Sure! Here's your kit:\n```json\n" + `{
 "hook": "I gave my phone a week off #detox",
 "hashtags": ["digital detox", "#SlowLiving", "#digitaldetox", "#Lusaka!"],
 "alt_text": "A phone face down on a wooden table next to a cup of tea and an open notebook",
 "posts": {
  "x": {"body": "Seven days, no feeds. I read 3 books, slept better and finally finished the garden. Here is everything I learned along the way, the good parts and the hard parts, and what I would do differently next time if I did it again. Join me @blaze_app"},
  "instagram": {"body": "Seven days without feeds.\n\nI read three books and slept better.\n#ignored #tags"},
  "reddit": {"title": "I took a week off my phone, here's what changed", "body": "Long story short: more sleep, more books."},
  "youtube_shorts": {"title": "A week without my phone", "body": "What happened when I put my phone away for seven days."}
 }
}` + "\n```"

func ideaReq() Request {
	return Request{Idea: "A week without my phone. Join me @blaze_app and read more at https://blaze.test/detox", Language: "en",
		Platforms: []string{"x", "instagram", "reddit", "youtube_shorts"}}
}

func TestMakeIdea(t *testing.T) {
	s := &stub{replies: []string{ideaReply}}
	k, note, err := Make(context.Background(), ideaReq(), s.ask)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.calls) != 1 || s.calls[0].photo != "" || strings.Contains(s.calls[0].system, "focus") {
		t.Fatalf("calls %+v", s.calls)
	}
	if note != "" {
		t.Fatalf("note %q", note)
	}
	if p := Validate(k); len(p) > 0 {
		t.Fatalf("invalid: %v", p)
	}
	b, _ := json.Marshal(k)
	if _, p := Parse(b); len(p) > 0 {
		t.Fatalf("JSON round trip: %v", p)
	}
	if k.Hook != "I gave my phone a week off" {
		t.Fatalf("hook %q", k.Hook)
	}
	if !reflect.DeepEqual(k.Hashtags, []string{"#digitaldetox", "#SlowLiving", "#Lusaka"}) {
		t.Fatalf("hashtags %v", k.Hashtags)
	}
	x := k.Platforms["x"].Caption
	if XLen(x) > 280 || !strings.HasPrefix(x, k.Hook) || !strings.Contains(x, "https://blaze.test/detox") || !strings.Contains(x, "@blaze_app") {
		t.Fatalf("x caption (%d): %q", XLen(x), x)
	}
	ig := k.Platforms["instagram"].Caption
	if strings.Contains(ig, "#ignored") || !strings.HasSuffix(ig, "#digitaldetox #SlowLiving #Lusaka") {
		t.Fatalf("instagram %q", ig)
	}
	rd := k.Platforms["reddit"]
	if rd.Title == "" || strings.Contains(rd.Caption, "#") || !strings.Contains(rd.Caption, "https://blaze.test/detox") {
		t.Fatalf("reddit %+v", rd)
	}
	yt := k.Platforms["youtube_shorts"]
	if Len(yt.Caption) > 100 || yt.Description == "" {
		t.Fatalf("shorts %+v", yt)
	}
	if k.Source == nil || k.Source.Width != 1080 || k.Source.Height != 1920 || len(k.Crops) != 4 {
		t.Fatalf("idea frame %+v %+v", k.Source, k.Crops)
	}
	if k.Idea != ideaReq().Idea {
		t.Fatal("idea not kept as given")
	}
}

func writePhoto(t *testing.T, w, h int) string {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x % 256), uint8(y % 256), 120, 255})
		}
	}
	p := filepath.Join(t.TempDir(), "Market Day.png")
	f, _ := os.Create(p)
	png.Encode(f, img)
	f.Close()
	return p
}

func TestMakePhotoAndExport(t *testing.T) {
	photo := writePhoto(t, 1200, 800)
	reply := `{"hook":"Saturday colours at the market","hashtags":["#market","#Zambia","#streetphotography"],
"alt_text":"Stalls of bright fabric and baskets of tomatoes under a blue sky, a woman smiling at the camera on the right",
"focus":{"x":0.8,"y":0.4},"posts":{"instagram":{"body":"Every Saturday the market wakes up in colour."},"threads":{"body":"Market mornings > everything."}}}`
	s := &stub{replies: []string{reply}}
	r := Request{Photo: photo, Width: 1200, Height: 800, Language: "en", Platforms: []string{"instagram", "threads", "instagram"}}
	k, _, err := Make(context.Background(), r, s.ask)
	if err != nil {
		t.Fatal(err)
	}
	if s.calls[0].photo != photo || !strings.Contains(s.calls[0].system, "focus") || !strings.Contains(s.calls[0].user, "1200x800") {
		t.Fatalf("photo not sent: %+v", s.calls[0])
	}
	if p := Validate(k); len(p) > 0 {
		t.Fatalf("invalid: %v", p)
	}
	if len(k.Platforms) != 2 || k.Source.Width != 1200 || k.Source.Focus.X != 0.8 {
		t.Fatalf("kit %+v", k)
	}
	sq := k.Crops[0] // 1:1 = 800x800 centred at x=960, clamped to 400
	if sq.Ratio != "1:1" || sq.W != 800 || sq.X != 400 {
		t.Fatalf("square crop %+v", sq)
	}
	dir := t.TempDir()
	base := Slug(k.Idea, photo)
	if base != "market-day" {
		t.Fatalf("slug %q", base)
	}
	files, saved, err := Export(dir, base, k, photo)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 6 {
		t.Fatalf("files %v", files)
	}
	for _, c := range saved.Crops {
		f, err := os.Open(filepath.Join(dir, c.File))
		if err != nil {
			t.Fatal(err)
		}
		cfg, err := jpeg.DecodeConfig(f)
		f.Close()
		if err != nil || cfg.Width != c.W || cfg.Height != c.H {
			t.Fatalf("%s: %v %dx%d", c.File, err, cfg.Width, cfg.Height)
		}
	}
	b, _ := os.ReadFile(filepath.Join(dir, base+"-kit.json"))
	sk, p := Parse(b)
	if len(p) > 0 || sk.Crops[1].File != "market-day-4x5.jpg" {
		t.Fatalf("saved kit: %v %+v", p, sk.Crops)
	}
	txt, _ := os.ReadFile(filepath.Join(dir, base+"-captions.txt"))
	if !strings.Contains(string(txt), "=== Threads ===") {
		t.Fatalf("captions: %s", txt)
	}
	if k.Crops[0].File != "" {
		t.Fatal("Export changed the caller's kit")
	}
}

func TestMakeRetriesThenFills(t *testing.T) {
	// Not JSON, then JSON that still lacks hashtags and a platform: the retry names the problem,
	// and the last try fills the gaps so the kit is still valid.
	s := &stub{replies: []string{"I can't do JSON today", `{"hook":"Fresh bread at 5am","alt_text":"","posts":{"x":{"body":"Out of the oven."}}}`}}
	r := Request{Idea: "Our bakery opens at 5am with fresh sourdough", Language: "fr", Platforms: []string{"x", "linkedin"}}
	k, note, err := Make(context.Background(), r, s.ask)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.calls) != 2 || !strings.Contains(s.calls[1].user, "no JSON object") {
		t.Fatalf("retry didn't name the problem: %+v", s.calls)
	}
	if !strings.Contains(s.calls[0].system, "French (fr)") || k.Language != "fr" {
		t.Fatal("language")
	}
	if p := Validate(k); len(p) > 0 {
		t.Fatalf("invalid: %v", p)
	}
	if note == "" || len(k.Hashtags) != 3 || k.Platforms["linkedin"].Caption == "" {
		t.Fatalf("fill: %q %+v", note, k)
	}
}

func TestMakeFailures(t *testing.T) {
	if _, _, err := Make(context.Background(), Request{Platforms: []string{"x"}}, (&stub{}).ask); err == nil {
		t.Fatal("empty request accepted")
	}
	if _, _, err := Make(context.Background(), Request{Idea: "hi", Platforms: []string{"myspace"}}, (&stub{}).ask); err == nil {
		t.Fatal("no platform accepted")
	}
	s := &stub{err: errors.New("connection refused")}
	if _, _, err := Make(context.Background(), Request{Idea: "hi", Platforms: []string{"x"}}, s.ask); !errors.Is(err, ErrModel) || len(s.calls) != 2 {
		t.Fatalf("model down: %v (%d calls)", err, len(s.calls))
	}
	s = &stub{replies: []string{"nope"}}
	if _, _, err := Make(context.Background(), Request{Idea: "hi", Platforms: []string{"x"}}, s.ask); err == nil {
		t.Fatal("garbage twice gave a kit")
	}
}

func TestCaptionLimitsEveryPlatform(t *testing.T) {
	long := strings.Repeat("This is a long sentence about the post. ", 1200) // ~48k characters
	var ps []string
	posts := map[string]ReplyPost{}
	for _, p := range Platforms {
		ps = append(ps, p.ID)
		posts[p.ID] = ReplyPost{Title: long[:500], Body: long}
	}
	r := Request{Idea: "Launch day {product} at https://blaze.test/l", Language: "en", Platforms: ps}
	k := Assemble(r, &Reply{Hook: "Launch day is here", Hashtags: []string{"#a1", "#b2", "#c3"}, AltText: "x", Posts: posts})
	if p := Validate(k); len(p) > 0 {
		t.Fatalf("%v", p)
	}
	for _, id := range []string{"x", "threads", "instagram"} {
		if !strings.Contains(k.Platforms[id].Caption, "{product}") || !strings.Contains(k.Platforms[id].Caption, "https://blaze.test/l") {
			t.Fatalf("%s lost a token", id)
		}
	}
}
