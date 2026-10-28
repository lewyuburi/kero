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
