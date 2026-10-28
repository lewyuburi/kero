package main

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"kero/internal/proto"
)

func TestSnakeMovesTurnsAndDiesOnTheWall(t *testing.T) {
	g := newSnake(1)
	head := g.body[0]
	if g.Interval() != 0 {
		t.Fatal("a game that has not begun steps")
	}
	g.Key(keyAction)
	g.Step()
	if g.body[0] != (point{head.x + 1, head.y}) || len(g.body) != 3 {
		t.Errorf("after a step the head is %v, body %d long", g.body[0], len(g.body))
	}
	// Not back on itself.
	g.Key(keyLeft)
	g.Step()
	if g.dir != (point{1, 0}) {
		t.Errorf("turned back on itself: %v", g.dir)
	}
	g.Key(keyUp)
	g.Step()
	if g.dir != (point{0, -1}) {
		t.Errorf("did not turn up: %v", g.dir)
	}
	for i := 0; i < g.h && !g.over; i++ {
		g.Step()
	}
	if !g.over {
		t.Error("it went through the wall")
	}
	g.Key(keyAction)
	if g.over || g.started || len(g.body) != 3 {
		t.Errorf("it did not start over: over %v, started %v, %d long", g.over, g.started, len(g.body))
	}
}

func TestSnakeEatsAndGrows(t *testing.T) {
	g := newSnake(1)
	g.Key(keyAction)
	g.food = point{g.body[0].x + 1, g.body[0].y}
	g.Step()
	if g.score != 1 || len(g.body) != 4 {
		t.Errorf("score %d, %d long after eating", g.score, len(g.body))
	}
	if contains(g.body, g.food) {
		t.Errorf("the food %v is under the snake", g.food)
	}
	if g.Interval() >= 140*1e6 {
		t.Errorf("it did not speed up: %v", g.Interval())
	}
}

func TestSnakeDiesOnItself(t *testing.T) {
	g := newSnake(1)
	g.body = []point{{5, 5}, {5, 6}, {6, 6}, {6, 5}, {6, 4}, {5, 4}}
	g.dir, g.queued = point{0, -1}, point{0, -1}
	g.food = point{0, 0}
	g.started = true
	g.Key(keyLeft) // into (4,5): free
	g.Step()
	if g.over {
		t.Fatal("died on an empty cell")
	}
	g.body = []point{{5, 5}, {5, 6}, {6, 6}, {6, 5}, {6, 4}, {5, 4}}
	g.dir, g.queued = point{0, -1}, point{0, -1}
	g.Step() // into (5,4), which the tail is leaving: allowed
	if g.over {
		t.Fatal("died on the tail's cell as it leaves")
	}
	g.body = []point{{5, 5}, {4, 5}, {4, 4}, {5, 4}, {6, 4}, {6, 5}, {6, 6}}
	g.dir, g.queued = point{1, 0}, point{0, -1}
	g.Step() // into (5,4), the body
	if !g.over {
		t.Error("went through itself")
	}
}

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
	if sn := play.g.(*snake); !sn.started {
		t.Error("an arrow did not begin the game")
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

func TestSnakeBonusesComeAndGo(t *testing.T) {
	g := newSnake(1)
	g.Key(keyAction)
	// One that is not taken goes when its time is up.
	g.bonus = &bonus{at: point{0, 0}, kind: bonusCherry, ttl: 3}
	for i := 0; i < 3; i++ {
		g.Step()
	}
	if g.bonus != nil {
		t.Errorf("a bonus out of time is still there: %+v", g.bonus)
	}
	// One that is taken is worth its points, and the snake grows.
	g = newSnake(1)
	g.Key(keyAction)
	length := len(g.body)
	g.bonus = &bonus{at: point{g.body[0].x + 1, g.body[0].y}, kind: bonusStar, ttl: 20}
	g.Step()
	if g.score != bonusPoints[bonusStar] || g.bonus != nil || len(g.body) != length+1 {
		t.Errorf("after the star: score %d, bonus %v, %d long (was %d)", g.score, g.bonus, len(g.body), length)
	}
	// A clock slows the game down for a while, and that ends.
	g = newSnake(1)
	g.Key(keyAction)
	fast := g.Interval()
	g.bonus = &bonus{at: point{g.body[0].x + 1, g.body[0].y}, kind: bonusClock, ttl: 20}
	g.Step()
	if g.Interval() <= fast {
		t.Errorf("a clock did not slow it down: %v, was %v", g.Interval(), fast)
	}
	g.slow = 1
	g.Step()
	if g.slow != 0 {
		t.Errorf("slow-mo did not run out: %d", g.slow)
	}
}

// Over many steps, a bonus shows up on a free cell, and never on the snake
// or the fruit.
func TestSnakeBonusesShowUpOnFreeCells(t *testing.T) {
	g := newSnake(7)
	g.Key(keyAction)
	g.score = 1
	seen := 0
	for i := 0; i < 4000; i++ {
		g.bonus = nil
		g.tickBonus()
		if b := g.bonus; b != nil {
			seen++
			if contains(g.body, b.at) || b.at == g.food {
				t.Fatalf("a bonus at %v is on the snake or its fruit", b.at)
			}
		}
	}
	// 2 in a hundred: about 80 of 4000.
	if seen < 40 || seen > 140 {
		t.Errorf("%d bonuses in 4000 chances, want about 80", seen)
	}
}
