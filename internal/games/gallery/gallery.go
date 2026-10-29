// Package gallery is the game that shows the others: tiles of the games there
// are, which a player moves over with the arrows or the mouse and chooses
// one of. It is a game of the catalog like the rest, and the one that "Games"
// in the command palette opens; it is not itself in what it shows.
package gallery

import (
	"time"

	"kero/internal/games"
	"kero/internal/pixel"
)

func init() {
	games.Register(games.Kind{
		ID: "gallery", Title: "Games",
		New:    func() games.Game { return newGallery(games.Kinds()) },
		Draw:   draw,
		Hidden: true,
	})
}

// The tiles: columns of them, 52 by 60 pixels with 8 between, in the middle of
// the area under the band at the top. Eight to a page.
const (
	cols, rows   = 4, 2
	perPage      = cols * rows
	tileW, tileH = 52, 60
	gap          = 8
)

// originX and originY are where the first tile's top left is: the tiles in
// the middle of the area.
const (
	originX = (games.Width - (cols*tileW + (cols-1)*gap)) / 2
	originY = games.HUDHeight + (games.AreaHeight-(rows*tileH+(rows-1)*gap))/2 - 4
)

type gallery struct {
	kinds   []games.Kind // the games shown, in the order they were registered
	sel     int
	chosen  string
	pressed int // the tile the mouse went down on, -1 for none
}

func newGallery(all []games.Kind) *gallery {
	g := &gallery{pressed: -1}
	for _, k := range all {
		if !k.Hidden {
			g.kinds = append(g.kinds, k)
		}
	}
	return g
}

func (g *gallery) Interval() time.Duration { return 0 }
func (g *gallery) Step()                   {}

// Chosen is the game chosen, once.
func (g *gallery) Chosen() string {
	id := g.chosen
	g.chosen = ""
	return id
}

func (g *gallery) Status() (score, hint string) {
	if len(g.kinds) == 0 {
		return "GAMES", ""
	}
	return "GAMES", "ENTER TO PLAY"
}

// tileRect is where the tile of a game is on the screen, on the page it is on.
func tileRect(i int) (x, y, w, h int) {
	i %= perPage
	return originX + (i%cols)*(tileW+gap), originY + (i/cols)*(tileH+gap), tileW, tileH
}

// tileAt is the game whose tile is at a point of the screen, -1 for none.
func (g *gallery) tileAt(px, py int) int {
	page := g.sel / perPage
	for i := page * perPage; i < min((page+1)*perPage, len(g.kinds)); i++ {
		x, y, w, h := tileRect(i)
		if px >= x && py >= y && px < x+w && py < y+h {
			return i
		}
	}
	return -1
}

func (g *gallery) Key(k games.Key) {
	if len(g.kinds) == 0 {
		return
	}
	n := len(g.kinds)
	switch k {
	case games.Left:
		g.sel = (g.sel + n - 1) % n
	case games.Right:
		g.sel = (g.sel + 1) % n
	case games.Up:
		if g.sel-cols >= 0 {
			g.sel -= cols
		}
	case games.Down:
		if g.sel+cols < n {
			g.sel += cols
		}
	case games.Action, games.Restart:
		g.chosen = g.kinds[g.sel].ID
	}
}

// Pointer moves the selection under the mouse, and a click that goes down and
// comes up on one tile chooses it.
func (g *gallery) Pointer(p games.Pointer) {
	i := g.tileAt(p.X, p.Y)
	switch p.Kind {
	case games.PointerMove:
		if i >= 0 {
			g.sel = i
		}
	case games.PointerDown:
		g.pressed = i
	case games.PointerUp:
		if i >= 0 && i == g.pressed {
			g.sel, g.chosen = i, g.kinds[i].ID
		}
		g.pressed = -1
	case games.PointerLeave:
		g.pressed = -1
	}
}

func draw(g games.Game, cv *pixel.Canvas) {
	ga := g.(*gallery)
	cv.Clear()
	font := pixel.FontNamed("sans5x7")
	page := ga.sel / perPage
	for slot := 0; slot < perPage; slot++ {
		i := page*perPage + slot
		x, y, w, h := tileRect(slot)
		cv.Frame(x, y, w, h, 2)
		if i >= len(ga.kinds) {
			// A place for a game that is not here yet.
			cv.Dither(x+2, y+2, w-4, h-4, 0.12, true)
			continue
		}
		k := ga.kinds[i]
		if k.Icon != nil {
			cv.Draw(k.Icon, x+(w-k.Icon.W)/2, y+8, pixel.Copy)
		} else {
			cv.Text(font, x+(w-font.Width("?"))/2, y+16, "?", pixel.Copy)
		}
		title := fit(font, k.Title, w-8)
		cv.Text(font, x+(w-font.Width(title))/2, y+h-14, title, pixel.Copy)
		if i == ga.sel {
			// The tile chosen is the other way about, as a handheld's menu does.
			cv.Invert(x, y, w, h)
			// Its corners are chipped, and stay so.
			for _, c := range [][2]int{{x, y}, {x + w - 1, y}, {x, y + h - 1}, {x + w - 1, y + h - 1}} {
				cv.Set(c[0], c[1], false)
			}
		}
	}
	// Dots for the pages, where there are more than one.
	pages := (len(ga.kinds) + perPage - 1) / perPage
	if pages > 1 {
		x := (games.Width - (pages*6 - 2)) / 2
		for p := 0; p < pages; p++ {
			cv.Rect(x+p*6, games.Height-8, 4, 4, p == page)
			if p != page {
				cv.Rect(x+p*6+1, games.Height-7, 2, 2, false)
				cv.Frame(x+p*6, games.Height-8, 4, 4, 1)
			}
		}
	}
}

// fit is a title cut to the room it has.
func fit(f *pixel.Font, s string, room int) string {
	r := []rune(s)
	for len(r) > 1 && f.Width(string(r)) > room {
		r = r[:len(r)-1]
	}
	return string(r)
}
