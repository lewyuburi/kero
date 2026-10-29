package snake

import (
	"testing"
	"time"

	"kero/internal/games"
	"kero/internal/pixel"
)

func TestSnakeMovesTurnsAndDiesOnTheWall(t *testing.T) {
	g := newSnake(1)
	head := g.body[0]
	if g.Interval() != 0 {
		t.Fatal("a game that has not begun steps")
	}
	g.Key(games.Action)
	g.Step()
	if g.body[0] != (point{head.x + 1, head.y}) || len(g.body) != 3 {
		t.Errorf("after a step the head is %v, body %d long", g.body[0], len(g.body))
	}
	// Not back on itself.
	g.Key(games.Left)
	g.Step()
	if g.dir != (point{1, 0}) {
		t.Errorf("turned back on itself: %v", g.dir)
	}
	g.Key(games.Up)
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
	g.Key(games.Action)
	if g.over || g.started || len(g.body) != 3 {
		t.Errorf("it did not start over: over %v, started %v, %d long", g.over, g.started, len(g.body))
	}
}

func TestSnakeEatsAndGrows(t *testing.T) {
	g := newSnake(1)
	g.Key(games.Action)
	g.food = point{g.body[0].x + 1, g.body[0].y}
	g.Step()
	if g.score != 1 || len(g.body) != 4 {
		t.Errorf("score %d, %d long after eating", g.score, len(g.body))
	}
	if contains(g.body, g.food) {
		t.Errorf("the food %v is under the snake", g.food)
	}
	if g.Interval() >= 170*time.Millisecond {
		t.Errorf("it did not speed up: %v", g.Interval())
	}
}

func TestSnakeDiesOnItself(t *testing.T) {
	g := newSnake(1)
	g.body = []point{{5, 5}, {5, 6}, {6, 6}, {6, 5}, {6, 4}, {5, 4}}
	g.dir, g.queued = point{0, -1}, point{0, -1}
	g.food = point{0, 0}
	g.started = true
	g.Key(games.Left) // into (4,5): free
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

func TestSnakeBonusesComeAndGo(t *testing.T) {
	g := newSnake(1)
	g.Key(games.Action)
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
	g.Key(games.Action)
	length := len(g.body)
	g.bonus = &bonus{at: point{g.body[0].x + 1, g.body[0].y}, kind: bonusStar, ttl: 20}
	g.Step()
	if g.score != bonusPoints[bonusStar] || g.bonus != nil || len(g.body) != length+1 {
		t.Errorf("after the star: score %d, bonus %v, %d long (was %d)", g.score, g.bonus, len(g.body), length)
	}
	// A clock slows the game down for a while, and that ends.
	g = newSnake(1)
	g.Key(games.Action)
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
	g.Key(games.Action)
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

// Dying shakes the screen for a few frames, and a bonus taken flashes it;
// both go as the frames do.
func TestSnakeEffectsRunOut(t *testing.T) {
	g := newSnake(1)
	g.Key(games.Action)
	if g.Animating() {
		t.Fatal("a screen that is still is animating")
	}
	g.bonus = &bonus{at: point{g.body[0].x + 1, g.body[0].y}, kind: bonusCherry, ttl: 9}
	g.Step()
	if !g.Animating() || g.flash != flashFrames {
		t.Errorf("a bonus taken did not flash: %d", g.flash)
	}
	for i := 0; i < flashFrames; i++ {
		g.Frame()
	}
	if g.Animating() {
		t.Error("the flash did not end")
	}
	g.Key(games.Up)
	for i := 0; i < g.h+1 && !g.over; i++ {
		g.Step()
	}
	if !g.over || g.shake != shakeFrames {
		t.Fatalf("it did not die shaking: over %v, shake %d", g.over, g.shake)
	}
	for i := 0; i < shakeFrames; i++ {
		g.Frame()
	}
	if g.Animating() {
		t.Error("the shake did not end")
	}
}

// The sprites of the snake are all there, cell-sized: a head and a tail for
// each way it can go, and what it eats.
func TestSnakeSpritesAreWhole(t *testing.T) {
	names := []string{"body", "body2", "apple", "cherry", "star", "clock"}
	for _, d := range []string{"right", "down", "left", "up"} {
		names = append(names, "head-"+d, "tail-"+d)
		if direction(map[string]point{"right": {1, 0}, "down": {0, 1}, "left": {-1, 0}, "up": {0, -1}}[d]) != d {
			t.Errorf("direction %s is not named so", d)
		}
	}
	for _, n := range names {
		want := sprite // the snake has room round it
		switch n {
		case "apple", "cherry", "star", "clock":
			want = Cell // what it eats fills its cell
		}
		img := sheet.Image(n)
		if img == nil || img.W != want || img.H != want {
			t.Errorf("sprite %q is missing or not %d by %d", n, want, want)
		}
	}
	if sprite != Cell-2 {
		t.Errorf("a sprite is %d in a cell of %d: no pixel of room all round", sprite, Cell)
	}
	if k, ok := games.Lookup("snake"); !ok || k.Icon == nil || k.Icon.W != 32 || k.Icon.H != 32 {
		t.Error("the snake's icon is not 32 by 32")
	}
	// The board fits the screen.
	if w, h := origin+Cols*Cell+7, top+Rows*Cell+7; w > games.Width || h > games.Height {
		t.Errorf("the board with its frame and shadow is %dx%d, on a screen of %dx%d", w, h, games.Width, games.Height)
	}
}

// Drawn, the snake leaves a pixel of room in each cell round what it is, and its
// segments are joined into one band.
func TestSnakeLeavesRoomAndIsJoined(t *testing.T) {
	g := newSnake(1)
	g.food = point{0, 0}
	g.bonus = nil
	cv := pixel.NewCanvas(games.Width, games.Height)
	draw(g, cv)
	// Three segments in a row on the middle line: between the first and the
	// second there is ink across the gap of the cells' edges, and there is a pixel
	// of paper above and below the band, in the cells.
	head, second := g.body[0], g.body[1]
	hx, hy := origin+head.x*Cell, top+head.y*Cell
	sx := origin + second.x*Cell
	for dy := pad; dy < Cell-pad; dy++ {
		if !cv.Get(hx-1, hy+dy) || !cv.Get(hx, hy+dy) || !cv.Get(sx+Cell-1, hy+dy) {
			t.Errorf("the band is broken at row %d between head and body", dy)
		}
	}
	// Room: the cell's first and last rows, over and under the band, are not ink
	// where the band runs (the dot of the board is on another row).
	if cv.Get(hx+Cell/2, hy) || cv.Get(hx+Cell/2, hy+Cell-1) {
		t.Error("the snake fills the cell, with no pixel of room")
	}
}
