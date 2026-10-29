package gallery

import (
	"testing"

	"kero/internal/games"
	"kero/internal/pixel"
)

func kinds(n int) []games.Kind {
	var ks []games.Kind
	for i := 0; i < n; i++ {
		ks = append(ks, games.Kind{ID: string(rune('a' + i)), Title: "Game " + string(rune('A'+i))})
	}
	return append(ks, games.Kind{ID: "hidden", Title: "Hidden", Hidden: true})
}

func TestGalleryShowsTheGamesButNotItself(t *testing.T) {
	g := newGallery(kinds(3))
	if len(g.kinds) != 3 {
		t.Fatalf("%d games shown, want 3 without the hidden one", len(g.kinds))
	}
	if _, ok := games.Lookup("gallery"); !ok {
		t.Fatal("the gallery is not a game of the catalog")
	}
	for _, k := range games.Kinds() {
		if k.ID == "gallery" && !k.Hidden {
			t.Error("the gallery shows itself")
		}
	}
	if g.Interval() != 0 {
		t.Error("the gallery has a clock")
	}
	if _, hint := g.Status(); hint == "" {
		t.Error("no hint for a gallery with games")
	}
	if _, hint := newGallery(nil).Status(); hint != "" {
		t.Errorf("a hint for no games: %q", hint)
	}
}

func TestGalleryKeysMoveAndChoose(t *testing.T) {
	g := newGallery(kinds(6))
	g.Key(games.Right)
	g.Key(games.Right)
	if g.sel != 2 {
		t.Errorf("right twice is %d", g.sel)
	}
	g.Key(games.Down)
	if g.sel != 2+cols && g.sel != 2 { // two rows of 4 on a page, 6 games
		t.Errorf("down is %d", g.sel)
	}
	g.Key(games.Left)
	g.Key(games.Up)
	if g.sel < 0 || g.sel >= 6 {
		t.Errorf("selection %d out of the games", g.sel)
	}
	// It wraps.
	g = newGallery(kinds(3))
	g.Key(games.Left)
	if g.sel != 2 {
		t.Errorf("left from the first is %d, want the last", g.sel)
	}
	g.Key(games.Right)
	if g.sel != 0 {
		t.Errorf("right from the last is %d, want the first", g.sel)
	}
	// Enter chooses, once.
	g.Key(games.Right)
	g.Key(games.Restart)
	if g.Chosen() != "b" || g.Chosen() != "" {
		t.Error("a choice was not made exactly once")
	}
	g.Key(games.Action)
	if g.Chosen() != "b" {
		t.Error("space does not choose")
	}
	// With no games there is nothing to do and nothing breaks.
	e := newGallery(nil)
	for _, k := range []games.Key{games.Left, games.Right, games.Up, games.Down, games.Action} {
		e.Key(k)
	}
	if e.Chosen() != "" {
		t.Error("a choice with no games")
	}
}

func TestGalleryMouseHoversAndClicks(t *testing.T) {
	g := newGallery(kinds(5))
	x1, y1, w, h := tileRect(1)
	g.Pointer(games.Pointer{Kind: games.PointerMove, X: x1 + w/2, Y: y1 + h/2})
	if g.sel != 1 {
		t.Errorf("hovering the second tile selects %d", g.sel)
	}
	// Between tiles nothing is chosen, and the selection stays.
	g.Pointer(games.Pointer{Kind: games.PointerMove, X: x1 + w + gap/2, Y: y1 + h/2})
	if g.sel != 1 {
		t.Errorf("hovering the space between selects %d", g.sel)
	}
	// A click that goes down and up on a tile chooses it.
	x3, y3, _, _ := tileRect(3)
	g.Pointer(games.Pointer{Kind: games.PointerDown, X: x3 + 4, Y: y3 + 4})
	g.Pointer(games.Pointer{Kind: games.PointerUp, X: x3 + 4, Y: y3 + 4})
	if g.Chosen() != "d" {
		t.Error("a click did not choose the tile")
	}
	// One that goes down on a tile and up on another, or leaves, does not.
	g.Pointer(games.Pointer{Kind: games.PointerDown, X: x1 + 4, Y: y1 + 4})
	g.Pointer(games.Pointer{Kind: games.PointerUp, X: x3 + 4, Y: y3 + 4})
	g.Pointer(games.Pointer{Kind: games.PointerDown, X: x1 + 4, Y: y1 + 4})
	g.Pointer(games.Pointer{Kind: games.PointerLeave})
	g.Pointer(games.Pointer{Kind: games.PointerUp, X: x1 + 4, Y: y1 + 4})
	if g.Chosen() != "" {
		t.Error("a drag between tiles, or out, chose one")
	}
	// An empty place is not a game.
	xe, ye, _, _ := tileRect(7)
	g.Pointer(games.Pointer{Kind: games.PointerDown, X: xe + 4, Y: ye + 4})
	g.Pointer(games.Pointer{Kind: games.PointerUp, X: xe + 4, Y: ye + 4})
	if g.Chosen() != "" {
		t.Error("an empty place was chosen")
	}
}

func TestGalleryPages(t *testing.T) {
	g := newGallery(kinds(perPage + 2))
	for i := 0; i < perPage; i++ {
		g.Key(games.Right)
	}
	if g.sel != perPage {
		t.Fatalf("selection %d, want %d", g.sel, perPage)
	}
	// The second page shows the game after the last of the first where the
	// first tile is.
	x, y, w, h := tileRect(perPage)
	if g.tileAt(x+w/2, y+h/2) != perPage {
		t.Errorf("the first tile of the second page is %d", g.tileAt(x+w/2, y+h/2))
	}
	// Of the first page, none can be clicked while the second is shown.
	if xa, ya, wa, ha := tileRect(1); g.tileAt(xa+wa/2, ya+ha/2) != perPage+1 {
		t.Errorf("the second tile of the second page is %d", g.tileAt(xa+wa/2, ya+ha/2))
	}
	// It draws, on every page, without leaving the screen.
	cv := pixel.NewCanvas(games.Width, games.Height)
	draw(g, cv)
	g.sel = 0
	draw(g, cv)
}

// A title is read whole when it can be: in the usual font if it fits in a
// tile, else in the small one, and only cut when neither does.
func TestTitlesAreReadWhole(t *testing.T) {
	room := tileW - 8
	for _, title := range []string{"Snake", "Black Ice", "Plants", "Tetris"} {
		f, got := fitTitle(title, room)
		if got != title {
			t.Errorf("%q is cut to %q in a tile %d wide", title, got, room)
		}
		if f.Width(got) > room {
			t.Errorf("%q does not fit: %d wide in %d", got, f.Width(got), room)
		}
	}
	f, got := fitTitle("Black Ice", room)
	if f.Name != "sans5x7" {
		t.Errorf("Black Ice, which fits in the usual font, is in %s", f.Name)
	}
	// One that does not fit in the usual font is in the small one.
	long := "Black Ice Zone"
	f, got = fitTitle(long, room)
	if f.Name != "tiny3x5" || got != long {
		t.Errorf("%q is %q in %s: want the small font, whole", long, got, f.Name)
	}
	// And one that fits in neither is cut, and still fits.
	f, got = fitTitle("A Very Long Name Of A Game", room)
	if got == "A Very Long Name Of A Game" || f.Width(got) > room || len(got) < 5 {
		t.Errorf("a very long title is %q (%d wide)", got, f.Width(got))
	}
}

func TestTilesFitTheScreenInTheArea(t *testing.T) {
	for i := 0; i < perPage; i++ {
		x, y, w, h := tileRect(i)
		if x < 0 || y < games.HUDHeight || x+w > games.Width || y+h > games.Height {
			t.Errorf("tile %d at %d,%d %dx%d is off the area", i, x, y, w, h)
		}
	}
	// And they are not on each other.
	for i := 0; i < perPage; i++ {
		for j := i + 1; j < perPage; j++ {
			ax, ay, aw, ah := tileRect(i)
			bx, by, bw, bh := tileRect(j)
			if ax < bx+bw && bx < ax+aw && ay < by+bh && by < ay+ah {
				t.Errorf("tiles %d and %d overlap", i, j)
			}
		}
	}
}

// The tile chosen is inverted, and keeps its chipped corners: nothing of it
// sticks out of where the tile is.
func TestSelectedTileKeepsItsCorners(t *testing.T) {
	g := newGallery(kinds(2))
	cv := pixel.NewCanvas(games.Width, games.Height)
	draw(g, cv)
	x, y, w, h := tileRect(0)
	for _, c := range [][2]int{{x, y}, {x + w - 1, y}, {x, y + h - 1}, {x + w - 1, y + h - 1}} {
		if cv.Get(c[0], c[1]) {
			t.Errorf("a corner of the tile chosen is ink at %v", c)
		}
	}
	// Inverted, its ground is ink and its frame is paper.
	if !cv.Get(x+3, y+3) || cv.Get(x+w/2, y) {
		t.Error("the tile chosen is not inverted")
	}
}
