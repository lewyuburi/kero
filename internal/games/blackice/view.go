package blackice

import (
	_ "embed"
	"strconv"
	"time"

	"kero/internal/games"
	"kero/internal/pixel"
)

func init() {
	games.Register(games.Kind{
		ID: "blackice", Title: "Black Ice",
		New:  func() games.Game { return New(time.Now().UnixNano()) },
		Draw: draw,
		Icon: sheet.Image("icon"),
	})
}

//go:embed blackice.sheet
var sheetSource string

var sheet = pixel.MustSheet(sheetSource)

var programSprites = [kinds]string{Pulse: "pulse", Dynamo: "dynamo", Wall: "wall", Mine: "mine", Cryo: "cryo"}

// roleSprites are what a card does, in a picture: it shoots, it makes power, it
// shields, it is a trap, it slows. The screen has no tooltips.
var roleSprites = [kinds]string{Pulse: "role-attack", Dynamo: "role-power", Wall: "role-shield", Mine: "role-trap", Cryo: "role-slow"}

// cardTips say what a card does, for the tooltip that shows when the pointer
// stays on it: its name, and what it does.
var cardTips = [kinds][2]string{
	Pulse:  {"PULSE", "SHOOTS DOWN ITS CONDUIT"},
	Dynamo: {"DYNAMO", "MAKES SPARKS: +25 WATTS"},
	Wall:   {"WALL", "BLOCKS MALWARE"},
	Mine:   {"MINE", "BLOWS UP WHAT STEPS ON IT"},
	Cryo:   {"CRYO", "SLOWS MALWARE DOWN"},
}

// frames is the sprite of a malware's kind, in the frame of its walk: two,
// which it goes between.
func frames(k enemyKind, tick int) string {
	name := malware[k].title
	if (tick/10)%2 == 1 {
		name += "2"
	}
	return name
}

// chip corners: the corners of a frame or a tile that is turned over keep
// their chips, as in the gallery.
func chip(cv *pixel.Canvas, x, y, w, h int) {
	for _, c := range [][2]int{{x, y}, {x + w - 1, y}, {x, y + h - 1}, {x + w - 1, y + h - 1}} {
		cv.Set(c[0], c[1], false)
	}
}

func draw(gm games.Game, cv *pixel.Canvas) {
	g := gm.(*game)
	cv.Clear()
	tiny := pixel.FontNamed("tiny3x5")
	sans := pixel.FontNamed("sans5x7")

	// The cards: the program, what it costs, and how soon it is ready. A card
	// that cannot be used is dimmed; the one in hand is turned over.
	for i := 0; i < int(kinds); i++ {
		x, y, w, h := cardRect(i)
		k := Kind(i)
		cv.Frame(x, y, w, h, 1)
		cv.Draw(sheet.Image(programSprites[k]), x+3, y+(h-spriteSize)/2, pixel.Copy)
		cost := itoa(k.Cost()) + string(wattSign)
		cv.Text(tiny, x+w-4-tiny.Width(cost), y+5, cost, pixel.Copy)
		cv.Draw(sheet.Image(roleSprites[k]), x+w-13, y+h-14, pixel.Copy)
		// How soon it is ready: a bar at the foot.
		if g.tick < g.cards[i].ready {
			left := g.cards[i].ready - g.tick
			span := programs[k].ready
			cv.Rect(x+2, y+h-4, (w-4)*(span-left)/span, 2, true)
		}
		if g.tick < g.cards[i].ready || g.watts < k.Cost() || (g.dragging && g.downCard == i) {
			cv.Dither(x+1, y+1, w-2, h-2, 0.45, false)
		}
		if i == g.selected {
			cv.Invert(x, y, w, h)
			chip(cv, x, y, w, h)
		} else if i == g.hoverCard {
			// A ring around it, which leaves the card as it is.
			outline(cv, x-2, y-2, w+4, h+4)
		}
	}

	// The sector: a frame, a dot in the middle of each tile, a dotted line
	// between the conduits, and a mark where they end at the core.
	cv.Frame(sectorX-3, sectorY-3, Cols*TileW+6, Rows*TileH+6, 2)
	for r := 0; r < Rows; r++ {
		for c := 0; c < Cols; c++ {
			cv.Set(sectorX+c*TileW+TileW/2, sectorY+r*TileH+TileH/2, true)
		}
		// The conduits are parted by a dotted line.
		if r > 0 {
			for x := sectorX; x < sectorX+Cols*TileW; x += 4 {
				cv.Set(x, sectorY+r*TileH, true)
			}
		}
	}
	cv.Rect(sectorX-3, sectorY-3, 3, Rows*TileH+6, true)
	for r := 0; r < Rows; r++ {
		cv.Rect(sectorX-3, sectorY+r*TileH+TileH/2-1, 3, 3, false)
	}

	// The programs, hurt ones dimmed, a mine not armed hollow.
	for r := 0; r < Rows; r++ {
		for c := 0; c < Cols; c++ {
			d := g.cells[r][c]
			if d == nil {
				continue
			}
			x, y := sectorX+c*TileW+spriteOff, sectorY+r*TileH+spriteOff
			cv.Draw(sheet.Image(programSprites[d.kind]), x, y, pixel.Copy)
			switch {
			case d.kind == Mine && d.armed > 0:
				cv.Dither(x, y, spriteSize, spriteSize, 0.5, false)
			case d.hp < programs[d.kind].hp/2:
				cv.Dither(x, y, spriteSize, spriteSize, 0.3, false)
			}
		}
	}

	// What the keyboard or the mouse is on, and the card in hand where it
	// would go.
	hc, hr := g.hoverC, g.hoverR
	if hc < 0 {
		hc, hr = g.curC, g.curR
	}
	if g.selected >= 0 && g.cells[hr][hc] == nil && !g.over && !g.won {
		x, y := sectorX+hc*TileW+spriteOff, sectorY+hr*TileH+spriteOff
		cv.Draw(sheet.Image(programSprites[g.selected]), x, y, pixel.Copy)
		cv.Dither(x, y, spriteSize, spriteSize, 0.5, false)
	}
	bracket(cv, sectorX+hc*TileW, sectorY+hr*TileH, TileW, TileH)

	// A card being dragged: its program, on the tile it is over if it can go
	// there, else where the pointer is, lighter as it is further from a place.
	if g.dragging && g.downCard >= 0 {
		img := sheet.Image(programSprites[g.downCard])
		if g.hoverC >= 0 && g.cells[g.hoverR][g.hoverC] == nil {
			x, y := sectorX+g.hoverC*TileW+spriteOff, sectorY+g.hoverR*TileH+spriteOff
			cv.Draw(img, x, y, pixel.Copy)
			cv.Dither(x, y, spriteSize, spriteSize, 0.25, false)
			bracket(cv, sectorX+g.hoverC*TileW, sectorY+g.hoverR*TileH, TileW, TileH)
		} else {
			x, y := g.dragX-spriteSize/2, g.dragY-spriteSize/2
			cv.Draw(img, x, y, pixel.Copy)
			cv.Dither(x, y, spriteSize, spriteSize, 0.5, false)
		}
	}

	// Malware, in the frame of its walk, slowed ones frosted.
	for _, e := range g.enemies {
		x := sectorX + int(e.x*TileW) - 4
		y := sectorY + e.lane*TileH + spriteOff
		cv.Draw(sheet.Image(frames(e.kind, g.tick+e.lane*3)), x, y, pixel.Copy)
		if e.slow > 0 {
			cv.Dither(x, y, spriteSize, spriteSize, 0.4, false)
		}
		if e.hp < malware[e.kind].hp/2 {
			cv.Dither(x, y, spriteSize, spriteSize, 0.2, false)
		}
	}
	for _, b := range g.bolts {
		name := "bolt"
		if b.cryo {
			name = "cryobolt"
		}
		cv.Draw(sheet.Image(name), sectorX+int(b.x*TileW), sectorY+b.lane*TileH+TileH/2-2, pixel.Copy)
	}
	for _, s := range g.sparks {
		// A spark about to go blinks, and all of them bob a little.
		if s.ttl < 3*perSec && (s.ttl/6)%2 == 0 {
			continue
		}
		cv.Draw(sheet.Image("spark"), int(s.x), int(s.y)+(g.tick/8)%2, pixel.Copy)
	}

	// Under the sector: the core, a chip for each that is left, and the waves.
	for i := 0; i < startCore; i++ {
		x, y := sectorX+i*13, infoY
		cv.Draw(sheet.Image("core"), x, y, pixel.Copy)
		if i >= g.core {
			cv.Dither(x, y, 10, 10, 0.7, false)
		}
	}
	barX, barY, barW := sectorX+48, infoY+1, Cols*TileW-48
	cv.Frame(barX, barY, barW, 8, 1)
	total := firstWave + waves*waveEvery
	cv.Rect(barX+1, barY+1, (barW-2)*min(g.tick, total)/total, 6, true)
	for w := 0; w <= waves; w++ {
		mx := barX + 1 + (barW-2)*min(firstWave+w*waveEvery, total)/total
		cv.Set(mx, barY+9, true)
	}

	if g.flash > 0 && g.flash%2 == 0 {
		cv.Invert(sectorX, sectorY+g.flashLane*TileH, Cols*TileW, TileH)
	}
	// What a card does, in a box over it, once the pointer has stayed on it.
	if g.tipShown() {
		x, y, w, _ := cardRect(g.hoverCard)
		tip := cardTips[g.hoverCard]
		cv.Tooltip(tiny, x+w/2, y, sectorY, tip[0], tip[1])
	}
	if g.over || g.won {
		// The sector washes out, and says how it ended in a box.
		cv.Dither(sectorX, sectorY, Cols*TileW, Rows*TileH, 0.55, false)
		msg := "SYSTEM DOWN"
		if g.won {
			msg = "SECTOR CLEARED"
		}
		w := sans.Width(msg) + 16
		x, y := sectorX+(Cols*TileW-w)/2, sectorY+(Rows*TileH-19)/2
		cv.Rect(x, y, w, 19, false)
		cv.Frame(x, y, w, 19, 2)
		cv.Text(sans, x+8, y+6, msg, pixel.Copy)
	}
	cv.SetShift(pixel.ShakeOffset(g.shake, 1))
}

// outline draws a one-pixel ring, and nothing inside it.
func outline(cv *pixel.Canvas, x, y, w, h int) {
	cv.Rect(x+1, y, w-2, 1, true)
	cv.Rect(x+1, y+h-1, w-2, 1, true)
	cv.Rect(x, y+1, 1, h-2, true)
	cv.Rect(x+w-1, y+1, 1, h-2, true)
}

// bracket marks a rectangle by its corners.
func bracket(cv *pixel.Canvas, x, y, w, h int) {
	const n = 5
	cv.Rect(x, y, n, 1, true)
	cv.Rect(x, y, 1, n, true)
	cv.Rect(x+w-n, y, n, 1, true)
	cv.Rect(x+w-1, y, 1, n, true)
	cv.Rect(x, y+h-1, n, 1, true)
	cv.Rect(x, y+h-n, 1, n, true)
	cv.Rect(x+w-n, y+h-1, n, 1, true)
	cv.Rect(x+w-1, y+h-n, 1, n, true)
}

func itoa(n int) string { return strconv.Itoa(n) }
