package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"kero/internal/games"
	_ "kero/internal/games/snake"
	"kero/internal/pixel"
	"kero/internal/proto"
	"kero/internal/settings"
	"kero/locales"
)

func TestGamePaneURL(t *testing.T) {
	p := proto.Pane{URL: proto.GameURL("snake")}
	if p.Game() != "snake" || !p.Browser() {
		t.Errorf("game %q, browser %v", p.Game(), p.Browser())
	}
	if (&proto.Pane{URL: "https://example.com"}).Game() != "" || (&proto.Pane{}).Game() != "" {
		t.Error("a page or a terminal plays a game")
	}
}

// A game pane shows its game, takes the keys while it has the keyboard, and
// is not a browser to the app: no page is made for it.
func TestGamePaneRenders(t *testing.T) {
	a := testApp(t)
	a.st.Sessions[0].Panes[1] = proto.Pane{ID: "b", URL: proto.GameURL("snake"), Status: proto.Idle, Progress: -1}
	tt := ui.NewTester(a.view, 1500, 800)
	tt.Frame()
	play := a.games["b"]
	if play == nil {
		t.Fatal("the pane has no game")
	}
	if a.webs["b"] != nil {
		t.Error("a page was made for a game")
	}
	a.paneRef("b").Focus()
	tt.Frame()
	tt.Frame()
	tt.Key(0, ui.KeyRight)
	tt.Frame()
	if _, hint := play.g.Status(); hint != "" {
		t.Errorf("an arrow did not begin the game: %q", hint)
	}
}

// With no session to split, a game starts one that is only the game.
func TestOpenGameWithoutSession(t *testing.T) {
	a := testApp(t)
	a.st.Sessions, a.selected = nil, ""
	a.local().conn = connReady
	got := make(chan proto.CreateParams, 1)
	cl, stop := projectClient(t, func(method string, raw json.RawMessage) (any, error) {
		var p proto.CreateParams
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		got <- p
		return proto.Session{ID: "s2"}, nil
	})
	a.local().cl = cl
	defer stop()
	a.openGame("snake")
	select {
	case p := <-got:
		if p.URL != proto.GameURL("snake") || p.Dir == "" || p.Split != "" {
			t.Errorf("created %+v, want a session of the game, in a folder, splitting nothing", p)
		}
	case <-time.After(time.Second):
		t.Fatal("no session was created for the game")
	}
}

// A screen keeps its proportions whatever the pane: filling, it takes all
// the room one way; whole, it is a whole number of the display's pixels to
// each of its own, and never larger than the room.
func TestScreenRect(t *testing.T) {
	for _, room := range []ui.Rect{{W: 700, H: 520}, {W: 520, H: 900}, {W: 1200, H: 300}, {W: 300, H: 300}, {X: 40, Y: 10, W: 611, H: 487}} {
		for _, dpr := range []float32{1, 2} {
			fill := screenRect(room, dpr, 256, 192, settings.GameScaleFill)
			if d := fill.W/fill.H - 256.0/192; d > 0.01 || d < -0.01 {
				t.Errorf("room %v dpr %v: fill is %vx%v, not 4:3", room, dpr, fill.W, fill.H)
			}
			if fill.W > room.W+0.01 || fill.H > room.H+0.01 || (room.W-fill.W > 2 && room.H-fill.H > 2) {
				t.Errorf("room %v dpr %v: fill is %vx%v, which does not fill it one way", room, dpr, fill.W, fill.H)
			}
			whole := screenRect(room, dpr, 256, 192, settings.GameScaleWhole)
			scale := whole.W / 256
			if n := scale * dpr; n != float32(int(n)) {
				t.Errorf("room %v dpr %v: whole scale %v is not a whole number of display pixels", room, dpr, scale)
			}
			if whole.W > fill.W+0.01 || whole.H > fill.H+0.01 {
				t.Errorf("room %v dpr %v: whole %vx%v is larger than fill %vx%v", room, dpr, whole.W, whole.H, fill.W, fill.H)
			}
			// In the middle, on whole display pixels.
			if x := (whole.X - room.X) * dpr; x != float32(int(x)) {
				t.Errorf("room %v dpr %v: whole starts between display pixels at %v", room, dpr, whole.X)
			}
		}
	}
	// On a display with two pixels to a DIP, whole goes in steps of a half.
	if got := screenRect(ui.Rect{W: 700, H: 600}, 2, 256, 192, settings.GameScaleWhole); got.W != 640 {
		t.Errorf("whole at 2 pixels to a DIP in 700 wide is %v wide, want 2.5x: 640", got.W)
	}
	if got := screenRect(ui.Rect{W: 700, H: 600}, 1, 256, 192, settings.GameScaleWhole); got.W != 512 {
		t.Errorf("whole at 1 pixel to a DIP in 700 wide is %v wide, want 2x: 512", got.W)
	}
}

// The games' settings are a page of their own, which the games flag offers:
// with it disabled the page is not in Settings, the search does not find it
// or its row, and the palette does not list it; enabled, all do, and the row
// is not on General.
func TestGamesPageFollowsFlag(t *testing.T) {
	s := settings.Default()
	if pageShown(s, "games") || !pageShown(s, "general") {
		t.Error("the games page is shown with its flag disabled, or General is not")
	}
	if r := searchResults("game screen", s); len(r) != 0 {
		t.Errorf("the search finds the games' settings with the flag disabled: %v", r)
	}
	if r := searchResults("games", s); r["games"] != nil {
		t.Errorf("the search finds the page with the flag disabled: %v", r)
	}
	s.SetFlag(settings.FlagGames, settings.StateEnabled)
	if !pageShown(s, "games") {
		t.Error("the games page is not shown with its flag enabled")
	}
	if got := searchResults("game screen", s)["games"]; len(got) != 1 {
		t.Errorf("the game screen is not found on Games: %v", searchResults("game screen", s))
	}

	a := testApp(t)
	a.ss.page = "general"
	tt := ui.NewTester(a.settingsView, 820, 640)
	tt.Frame()
	if tt.HasText("Games") {
		t.Error("a Games page in Settings with the flag disabled")
	}
	a.set.SetFlag(settings.FlagGames, settings.StateEnabled)
	tt.Frame()
	tt.Advance(time.Second)
	tt.Frame()
	if !tt.HasText("Games") {
		t.Errorf("no Games page with the flag enabled; texts %q", tt.Texts())
	}
	if tt.HasText("Game screen") {
		t.Error("the game screen is on General")
	}
	a.ss.page = "games"
	tt.Frame()
	tt.Advance(time.Second)
	tt.Frame()
	if !tt.HasText("Game screen") || !tt.HasText("Whole pixels") {
		t.Errorf("the Games page lacks the screen's setting; texts %q", tt.Texts())
	}
}

// Snap fills the pane, but is whole where whole keeps nine tenths of what
// filling takes: close to a whole scale it is whole, far from one it fills.
func TestScreenRectSnap(t *testing.T) {
	// 1 pixel to a DIP: 540 wide fits 2.11x, whole is 2x, which is 94.8% of it.
	near := ui.Rect{W: 540, H: 600}
	if got := screenRect(near, 1, 256, 192, settings.GameScaleSnap); got.W != 512 {
		t.Errorf("snap near a whole scale is %v wide, want the whole 512", got.W)
	}
	// 700 wide fits 2.73x, and whole 2x keeps 73%: it fills.
	far := ui.Rect{W: 700, H: 600}
	snap := screenRect(far, 1, 256, 192, settings.GameScaleSnap)
	fill := screenRect(far, 1, 256, 192, settings.GameScaleFill)
	if snap != fill || snap.W != 700 {
		t.Errorf("snap far from a whole scale is %v, want it to fill like %v", snap, fill)
	}
	// And it stays in proportion either way.
	for _, r := range []ui.Rect{near, far, {W: 300, H: 900}} {
		got := screenRect(r, 1, 256, 192, settings.GameScaleSnap)
		if d := got.W/got.H - 256.0/192; d > 0.02 || d < -0.02 {
			t.Errorf("room %v: snap is %vx%v, not 4:3", r, got.W, got.H)
		}
	}
}

func TestScreenPoint(t *testing.T) {
	// A screen of 256 by 192 shown at 2x, 40 in and 10 down in its box.
	rel := ui.Rect{X: 40, Y: 10, W: 512, H: 384}
	for _, tc := range []struct {
		x, y   float32
		px, py int
		ok     bool
	}{
		{40, 10, 0, 0, true},
		{41.9, 11.9, 0, 0, true},
		{42, 12, 1, 1, true},
		{40 + 511, 10 + 383, 255, 191, true},
		{39, 10, 0, 0, false},   // left of it
		{40, 9, 0, 0, false},    // above
		{552, 100, 0, 0, false}, // its right edge is outside
		{100, 394, 0, 0, false},
	} {
		px, py, ok := screenPoint(tc.x, tc.y, rel, 256, 192)
		if ok != tc.ok || (ok && (px != tc.px || py != tc.py)) {
			t.Errorf("point %v,%v: pixel %d,%d ok %v, want %d,%d %v", tc.x, tc.y, px, py, ok, tc.px, tc.py, tc.ok)
		}
	}
	if _, _, ok := screenPoint(5, 5, ui.Rect{}, 256, 192); ok {
		t.Error("a screen not yet drawn has a point")
	}
}

// pointerGame is a game that keeps the pointers it is told of.
type pointerGame struct{ events []games.Pointer }

func (g *pointerGame) Interval() time.Duration  { return 0 }
func (g *pointerGame) Step()                    {}
func (g *pointerGame) Key(games.Key)            {}
func (g *pointerGame) Status() (string, string) { return "", "" }
func (g *pointerGame) Pointer(p games.Pointer)  { g.events = append(g.events, p) }

var testPointerGame = &pointerGame{}

func init() {
	games.Register(games.Kind{ID: "test-pointer", Title: "Test", Hidden: true,
		New:  func() games.Game { return testPointerGame },
		Draw: func(games.Game, *pixel.Canvas) {}})
}

// The mouse reaches a game in pixels of its screen, whatever the scale: that
// it moved, went down, came up, left.
func TestPointerReachesTheGame(t *testing.T) {
	for _, mode := range []string{settings.GameScaleWhole, settings.GameScaleFill} {
		a := testApp(t)
		a.set.GameScale = mode
		testPointerGame.events = nil
		a.st.Sessions[0].Panes[1] = proto.Pane{ID: "b", URL: proto.GameURL("test-pointer"), Status: proto.Idle, Progress: -1}
		var box ui.Rect
		tt := ui.NewTester(func(c *ui.Context) {
			a.view(c)
			box = a.paneRef("b").Bounds(c)
		}, 1400, 800)
		for i := 0; i < 3; i++ {
			tt.Frame()
		}
		play := a.games["b"]
		if play == nil || play.rel.W == 0 {
			t.Fatalf("%s: the screen was not drawn: %+v", mode, play)
		}
		// The window coordinates of a pixel of the screen: its middle.
		at := func(px, py int) (float32, float32) {
			sx, sy := play.rel.W/float32(play.cv.W), play.rel.H/float32(play.cv.H)
			return box.X + 8 + play.rel.X + (float32(px)+0.5)*sx, box.Y + play.rel.Y + (float32(py)+0.5)*sy
		}
		x, y := at(100, 50)
		tt.Move(x, y)
		tt.Frame()
		tt.Press(x, y)
		tt.Frame()
		tt.Release(x, y)
		tt.Frame()
		ev := testPointerGame.events
		want := []games.Pointer{
			{Kind: games.PointerMove, X: 100, Y: 50},
			{Kind: games.PointerDown, X: 100, Y: 50},
			{Kind: games.PointerUp, X: 100, Y: 50},
		}
		if len(ev) < len(want) {
			t.Fatalf("%s: events %+v, want %+v", mode, ev, want)
		}
		for i, w := range want {
			if ev[i] != w {
				t.Errorf("%s: event %d is %+v, want %+v", mode, i, ev[i], w)
			}
		}
		// Off the screen, in the pane's paper, it leaves.
		tt.Move(box.X+box.W-2, box.Y+box.H-2)
		tt.Frame()
		if last := testPointerGame.events[len(testPointerGame.events)-1]; last.Kind != games.PointerLeave {
			t.Errorf("%s: the last event is %+v, want a leave", mode, last)
		}
	}
}

// The player of the gallery chooses a game and gets it in the same pane,
// whose new address the daemon is told.
func TestGalleryOpensTheGameChosen(t *testing.T) {
	a := testApp(t)
	a.local().conn = connReady
	got := make(chan proto.PageParams, 1)
	cl, stop := projectClient(t, func(method string, raw json.RawMessage) (any, error) {
		var p proto.PageParams
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		if method == "pane.page" {
			got <- p
		}
		return true, nil
	})
	a.local().cl = cl
	defer stop()
	a.st.Sessions[0].Panes[1] = proto.Pane{ID: "b", URL: proto.GameURL("gallery"), Status: proto.Idle, Progress: -1}
	tt := ui.NewTester(a.view, 1400, 700)
	for i := 0; i < 3; i++ {
		tt.Frame()
	}
	a.paneRef("b").Focus()
	tt.Frame()
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	tt.Frame()
	select {
	case p := <-got:
		if p.ID != "b" || p.URL != proto.GameURL("snake") {
			t.Errorf("the daemon was told %+v, want pane b at the snake", p)
		}
	case <-time.After(time.Second):
		t.Fatal("the daemon was not told of the game chosen")
	}
	if pane := a.st.Sessions[0].PaneByID("b"); pane.Game() != "snake" {
		t.Errorf("the pane plays %q", pane.Game())
	}
	if play := a.games["b"]; play == nil || play.kind.ID != "snake" {
		t.Errorf("the pane did not take the game: %+v", play)
	}
}

// The palette has one entry for games, Games, with the flag; and a game is
// opened by name, without the gallery, by the action pane.game.
func TestGamesInThePaletteAndByName(t *testing.T) {
	a := testApp(t)
	if items := a.gameItems(); len(items) != 0 {
		t.Errorf("games in the palette with the flag disabled: %d", len(items))
	}
	a.set.SetFlag(settings.FlagGames, settings.StateEnabled)
	items := a.gameItems()
	if len(items) != 1 || items[0].title != "Games" {
		t.Fatalf("palette entries %+v, want one, Games", items)
	}
	for _, word := range []string{"snake", "gallery", "play"} {
		if !strings.Contains(items[0].words, word) {
			t.Errorf("Games is not found by %q: %q", word, items[0].words)
		}
	}

	a.local().conn = connReady
	got := make(chan proto.CreateParams, 2)
	cl, stop := projectClient(t, func(method string, raw json.RawMessage) (any, error) {
		var p proto.CreateParams
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		got <- p
		return proto.Session{ID: "s1"}, nil
	})
	a.local().cl = cl
	defer stop()
	for args, want := range map[string]string{"": proto.GameURL("gallery"), "snake": proto.GameURL("snake")} {
		a.run("pane.game", map[string]string{"game": args})
		select {
		case p := <-got:
			if p.URL != want {
				t.Errorf("pane.game %q opened %q, want %q", args, p.URL, want)
			}
		case <-time.After(time.Second):
			t.Fatalf("pane.game %q opened nothing", args)
		}
	}
}

// Escape goes back from a game to the gallery, in the same pane, whose address
// the daemon is told; in the gallery it does nothing.
func TestEscapeGoesBackToTheGallery(t *testing.T) {
	a := testApp(t)
	a.local().conn = connReady
	got := make(chan proto.PageParams, 2)
	cl, stop := projectClient(t, func(method string, raw json.RawMessage) (any, error) {
		var p proto.PageParams
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		if method == "pane.page" {
			got <- p
		}
		return true, nil
	})
	a.local().cl = cl
	defer stop()
	a.st.Sessions[0].Panes[1] = proto.Pane{ID: "b", URL: proto.GameURL("snake"), Status: proto.Idle, Progress: -1}
	tt := ui.NewTester(a.view, 1400, 700)
	for i := 0; i < 3; i++ {
		tt.Frame()
	}
	a.paneRef("b").Focus()
	tt.Frame()
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	tt.Frame()
	select {
	case p := <-got:
		if p.ID != "b" || p.URL != proto.GameURL("gallery") {
			t.Errorf("the daemon was told %+v, want pane b at the gallery", p)
		}
	case <-time.After(time.Second):
		t.Fatal("Escape did not go back to the gallery")
	}
	if play := a.games["b"]; play == nil || play.kind.ID != "gallery" {
		t.Errorf("the pane did not take the gallery: %+v", play)
	}
	// In the gallery, Escape is nothing.
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	select {
	case p := <-got:
		t.Errorf("Escape in the gallery went somewhere: %+v", p)
	case <-time.After(150 * time.Millisecond):
	}
}

// A game's header has a button back to the gallery, which the gallery's does
// not, and it goes there.
func TestBackButtonInAGamesHeader(t *testing.T) {
	a := testApp(t)
	a.local().conn = connReady
	got := make(chan proto.PageParams, 1)
	cl, stop := projectClient(t, func(method string, raw json.RawMessage) (any, error) {
		var p proto.PageParams
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		if method == "pane.page" {
			got <- p
		}
		return true, nil
	})
	a.local().cl = cl
	defer stop()
	label := locales.PageGames() + " (Esc)"
	for _, url := range []string{proto.GameURL("gallery"), proto.GameURL("snake")} {
		a.st.Sessions[0].Panes[1] = proto.Pane{ID: "b", URL: url, Status: proto.Idle, Progress: -1}
		delete(a.games, "b")
		var box ui.Rect
		tt := ui.NewTester(func(c *ui.Context) {
			a.view(c)
			box = a.paneRef("card-b").Bounds(c)
		}, 1400, 700)
		tt.Frame()
		tt.Move(box.X+box.W/2, box.Y+20) // over the header: its buttons show
		tt.Frame()
		tt.Advance(time.Second)
		tt.Frame()
		if url == proto.GameURL("gallery") {
			if tt.HasText(label) {
				t.Error("the gallery has a button back to itself")
			}
			continue
		}
		if err := tt.Click(label); err != nil {
			t.Fatalf("no back button in a game's header: %v; texts %q", err, tt.Texts())
		}
		tt.Frame()
		tt.Frame()
	}
	select {
	case p := <-got:
		if p.URL != proto.GameURL("gallery") {
			t.Errorf("the button went to %q", p.URL)
		}
	case <-time.After(time.Second):
		t.Fatal("the button did not go back")
	}
}
