package main

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	_ "kero/internal/games/snake"
	"kero/internal/proto"
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
			fill := screenRect(room, dpr, 256, 192, false)
			if d := fill.W/fill.H - 256.0/192; d > 0.01 || d < -0.01 {
				t.Errorf("room %v dpr %v: fill is %vx%v, not 4:3", room, dpr, fill.W, fill.H)
			}
			if fill.W > room.W+0.01 || fill.H > room.H+0.01 || (room.W-fill.W > 2 && room.H-fill.H > 2) {
				t.Errorf("room %v dpr %v: fill is %vx%v, which does not fill it one way", room, dpr, fill.W, fill.H)
			}
			whole := screenRect(room, dpr, 256, 192, true)
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
	if got := screenRect(ui.Rect{W: 700, H: 600}, 2, 256, 192, true); got.W != 640 {
		t.Errorf("whole at 2 pixels to a DIP in 700 wide is %v wide, want 2.5x: 640", got.W)
	}
	if got := screenRect(ui.Rect{W: 700, H: 600}, 1, 256, 192, true); got.W != 512 {
		t.Errorf("whole at 1 pixel to a DIP in 700 wide is %v wide, want 2x: 512", got.W)
	}
}
