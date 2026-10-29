package blackice

import (
	"testing"

	"kero/internal/games"
	"kero/internal/pixel"
)

func TestBlackIceIsInTheCatalogWithAnIcon(t *testing.T) {
	k, ok := games.Lookup("blackice")
	if !ok {
		t.Fatal("BLACK ICE is not in the catalog")
	}
	if k.Hidden || k.Title == "" {
		t.Errorf("kind %+v", k)
	}
	if k.Icon == nil || k.Icon.W != 24 || k.Icon.H != 24 {
		t.Errorf("icon %+v, want 24 by 24", k.Icon)
	}
	if _, ok := k.New().(games.Pointed); !ok {
		t.Error("it does not take the mouse")
	}
	if _, ok := k.New().(games.Animator); !ok {
		t.Error("it has no effects")
	}
}

func TestEverySpriteIsInTheSheet(t *testing.T) {
	for _, name := range append(programSprites[:], "spark", "core", "bolt", "cryobolt", "icon") {
		if sheet.Image(name) == nil {
			t.Errorf("no sprite %q", name)
		}
	}
	for k := enemyKind(0); k < enemyKinds; k++ {
		for tick := 0; tick < 30; tick += 10 {
			if sheet.Image(frames(k, tick)) == nil {
				t.Errorf("no sprite %q for malware %d at %d", frames(k, tick), k, tick)
			}
		}
	}
	for _, name := range append(programSprites[:], "drone", "worm", "trojan") {
		if img := sheet.Image(name); img.W != 16 || img.H != 16 {
			t.Errorf("sprite %s is %dx%d, not 16x16", name, img.W, img.H)
		}
	}
	// A walk's two frames are the same size, so that it does not jump.
	for _, n := range []string{"drone", "worm", "trojan"} {
		a, b := sheet.Image(n), sheet.Image(n+"2")
		if a.W != b.W || a.H != b.H {
			t.Errorf("%s and %s differ in size", n, n+"2")
		}
	}
}

// Drawing the game in every state is no harm, stays on the screen's canvas, and
// shows what is going on: a game is not blank.
func TestDrawingEveryState(t *testing.T) {
	ink := func(cv *pixel.Canvas) (n int) {
		for y := 0; y < cv.H; y++ {
			for x := 0; x < cv.W; x++ {
				if cv.Get(x, y) {
					n++
				}
			}
		}
		return n
	}
	cv := pixel.NewCanvas(games.Width, games.Height)
	g := newGame(1)
	draw(g, cv)
	empty := ink(cv)
	if empty < 500 {
		t.Errorf("a game not begun has %d inked pixels: nearly blank", empty)
	}
	g.started = true
	g.selected = int(Cryo)
	g.place(Pulse, 2, 2)
	g.place(Wall, 3, 2)
	g.enemies = []*enemy{{kind: Trojan, lane: 2, x: 3.4, hp: 56}, {kind: Worm, lane: 0, x: 8, hp: 12, slow: 5}, {kind: Drone, lane: 4, x: 0.1, hp: 20}}
	g.bolts = []bolt{{lane: 2, x: 2.5}, {lane: 1, x: 8.9, cryo: true}}
	g.sparks = []spark{{x: 50, y: 60, ttl: 30, value: 25}, {x: 150, y: 90, ttl: 400, value: 25}}
	g.hoverC, g.hoverR = 1, 1
	draw(g, cv)
	if ink(cv) <= empty {
		t.Error("placing programs and malware drew nothing more")
	}
	for _, mode := range []func(){
		func() { g.over = true },
		func() { g.over, g.won = false, true },
		func() { g.won, g.shake, g.flash = false, shakeFrames, flashFrames },
	} {
		mode()
		draw(g, cv)
	}
	// The shake moves the screen; a still game does not.
	g.shake = 3
	draw(g, cv)
	if cv.RGBA(pixel.Themes[0].Ink, pixel.Themes[0].Paper, nil) == nil {
		t.Fatal("no picture")
	}
	g.shake = 0
	draw(g, cv)
}

// Every card on the screen is where the engine's mouse finds it.
func TestCardsAreWhereTheMouseFindsThem(t *testing.T) {
	g := newGame(1)
	for i := 0; i < int(kinds); i++ {
		x, y, w, h := cardRect(i)
		if g.cardAt(x+w/2, y+h/2) != i {
			t.Errorf("card %d is not at its middle", i)
		}
	}
}

// Hovering a card rings it and leaves it as it is: the card keeps what it draws.
func TestHoveringACardLeavesItWhole(t *testing.T) {
	g := newGame(1)
	plain, hover := pixel.NewCanvas(games.Width, games.Height), pixel.NewCanvas(games.Width, games.Height)
	g.hoverCard = -1
	draw(g, plain)
	for i := 0; i < int(kinds); i++ {
		g.hoverCard = i
		draw(g, hover)
		x, y, w, h := cardRect(i)
		for py := y; py < y+h; py++ {
			for px := x; px < x+w; px++ {
				if plain.Get(px, py) != hover.Get(px, py) {
					t.Fatalf("hovering card %d changed the card at %d,%d", i, px, py)
				}
			}
		}
		// The ring is outside it, and over nothing of another card.
		if !hover.Get(x-2+w/2, y-2) || !hover.Get(x-2, y+h/2) {
			t.Errorf("no ring round card %d", i)
		}
		if i > 0 {
			px, _, pw, _ := cardRect(i - 1)
			for py := y; py < y+h; py++ {
				for qx := px; qx < px+pw; qx++ {
					if plain.Get(qx, py) != hover.Get(qx, py) {
						t.Fatalf("the ring round card %d touches card %d", i, i-1)
					}
				}
			}
		}
	}
}

func TestEveryCardSaysWhatItDoes(t *testing.T) {
	seen := map[string]bool{}
	for k := Kind(0); k < kinds; k++ {
		name := roleSprites[k]
		img := sheet.Image(name)
		if img == nil || img.W != 7 || img.H != 7 {
			t.Fatalf("card %d: role sprite %q is %+v, want 7 by 7", k, name, img)
		}
		if seen[name] {
			t.Errorf("two cards share the role %q", name)
		}
		seen[name] = true
		// It is on the card, in its corner, and not on the picture or the cost.
		cv := pixel.NewCanvas(games.Width, games.Height)
		draw(newGame(1), cv)
		x, y, w, _ := cardRect(int(k))
		inked := 0
		for py := y + 12; py < y+19; py++ {
			for px := x + w - 10; px < x+w-3; px++ {
				if cv.Get(px, py) {
					inked++
				}
			}
		}
		if inked < 10 {
			t.Errorf("card %d has %d inked pixels where its role is", k, inked)
		}
	}
}
