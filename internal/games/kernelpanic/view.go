package kernelpanic

import (
	_ "embed"
	"strconv"
	"time"

	"kero/internal/games"
	"kero/internal/pixel"
)

func init() {
	games.Register(games.Kind{
		ID: "kernelpanic", Title: "Kernel Panic",
		// What it was called, so that a pane that was showing it still does.
		Aliases: []string{"blackice"},
		New:     func() games.Game { return New(time.Now().UnixNano()) },
		Draw:    draw,
		Icon:    sheet.Image("icon"),
	})
}

//go:embed kernelpanic.sheet
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
	switch g.screen {
	case screenMenu:
		drawMenu(g, cv)
		return
	case screenReward:
		drawReward(g, cv)
		return
	}
	tiny := pixel.FontNamed("tiny3x5")
	sans := pixel.FontNamed("sans5x7")

	// The cards: the program, what it costs, and how soon it is ready. A card
	// that cannot be used is dimmed; the one in hand is turned over.
	for i := 0; i < int(kinds); i++ {
		x, y, w, h := cardRect(i)
		k := Kind(i)
		if g.open[k] {
			cv.Frame(x, y, w, h, 1)
		}
		if !g.open[k] {
			// A program the player has not got yet: its place, dotted, with a lock.
			cv.Dither(x+1, y+1, w-2, h-2, closedShade, true)
			dashed(cv, x, y, w, h)
			cv.Rect(x+(w-14)/2, y+(h-14)/2, 14, 14, false)
			cv.Draw(sheet.Image("lock"), x+(w-12)/2, y+(h-12)/2, pixel.Copy)
			continue
		}
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
		if !g.lanes[r] {
			// Out of play: a lock where it begins, and no dots in its tiles, which
			// the conduits in play have: it is plain which is which.
			cv.Draw(sheet.Image("lock"), sectorX+(TileW-12)/2, sectorY+r*TileH+(TileH-12)/2, pixel.Copy)
		} else {
			// In play: framed where it meets a conduit out of play.
			if r > 0 && !g.lanes[r-1] {
				cv.Rect(sectorX, sectorY+r*TileH, Cols*TileW, 1, true)
			}
			if r < Rows-1 && !g.lanes[r+1] {
				cv.Rect(sectorX, sectorY+(r+1)*TileH-1, Cols*TileW, 1, true)
			}
		}
		for c := 0; c < Cols && g.lanes[r]; c++ {
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
			name := programSprites[d.kind]
			if d.kind == Dynamo {
				// Each dynamo a little out of step with the next, by where it is.
				name = dynamoSprite(d, g.tick+c*7+r*3)
			}
			cv.Draw(sheet.Image(name), x, y, pixel.Copy)
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
		// The card in hand over a tile: dimmed, and dimmed more where it cannot go.
		x, y := sectorX+hc*TileW+spriteOff, sectorY+hr*TileH+spriteOff
		cv.Draw(sheet.Image(programSprites[g.selected]), x, y, pixel.Copy)
		shade := float32(0.5)
		if g.cannotPlace(Kind(g.selected), hc, hr) != "" {
			shade = refusedShade
		}
		cv.Dither(x, y, spriteSize, spriteSize, shade, false)
	}
	bracket(cv, sectorX+hc*TileW, sectorY+hr*TileH, TileW, TileH)

	// A card being dragged: its program, on the tile it is over if it can go
	// there, else where the pointer is, lighter as it is further from a place.
	if g.dragging && g.downCard >= 0 {
		img := sheet.Image(programSprites[g.downCard])
		if g.hoverC >= 0 && g.cannotPlace(Kind(g.downCard), g.hoverC, g.hoverR) == "" {
			// A place where it can be dropped: it goes there, almost solid.
			x, y := sectorX+g.hoverC*TileW+spriteOff, sectorY+g.hoverR*TileH+spriteOff
			cv.Draw(img, x, y, pixel.Copy)
			cv.Dither(x, y, spriteSize, spriteSize, 0.25, false)
			bracket(cv, sectorX+g.hoverC*TileW, sectorY+g.hoverR*TileH, TileW, TileH)
		} else {
			// Where it cannot: it stays with the pointer, dimmed.
			x, y := g.dragX-spriteSize/2, g.dragY-spriteSize/2
			cv.Draw(img, x, y, pixel.Copy)
			cv.Dither(x, y, spriteSize, spriteSize, refusedShade, false)
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
	total := g.lv.length()
	cv.Rect(barX+1, barY+1, (barW-2)*min(g.tick, total)/total, 6, true)
	for w := 0; w <= len(g.lv.Waves); w++ {
		mx := barX + 1 + (barW-2)*min(g.lv.Start+w*g.lv.Every, total)/total
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

// drawMenu draws the sectors: a node for each, locked ones with a lock, cleared
// ones with the chips of their mark, the one chosen turned over; under them, what
// the one chosen has: its conduits, its malware, the programs the player has, and
// the one it gives.
func drawMenu(g *game, cv *pixel.Canvas) {
	sans := pixel.FontNamed("sans5x7")
	for i := range campaign {
		x, y, w, h := nodeRect(i)
		if !g.prog.open(i) {
			// A sector that is not open: dotted, with a lock and nothing more.
			cv.Dither(x+1, y+1, w-2, h-2, closedShade, true)
			dashed(cv, x, y, w, h)
			cv.Rect(x+(w-14)/2, y+(h-14)/2, 14, 14, false)
			cv.Draw(sheet.Image("lock"), x+(w-12)/2, y+(h-12)/2, pixel.Copy)
			if i == g.menuSel {
				dashed(cv, x-2, y-2, w+4, h+4)
			}
			continue
		}
		cv.Frame(x, y, w, h, 1)
		cv.Text(sans, x+(w-sans.Width(campaign[i].ID))/2, y+8, campaign[i].ID, pixel.Copy)
		// The mark: a chip for each earned, and the others hollow.
		earned := g.prog.Chips[campaign[i].ID]
		for c := 0; c < startCore; c++ {
			cx, cy := x+(w-startCore*10-(startCore-1)*3)/2+c*13, y+h-15
			cv.Rect(cx, cy, 10, 10, false)
			if c < earned {
				cv.Draw(sheet.Image("core"), cx, cy, pixel.Copy)
			} else {
				outline(cv, cx+1, cy+1, 8, 8)
			}
		}
		if i == g.menuSel {
			cv.Invert(x, y, w, h)
			chip(cv, x, y, w, h)
		}
	}
	lv := &campaign[g.menuSel]
	x0, y0 := menuX, panelY
	cv.Frame(x0, y0, panelW, panelH, 1)
	cv.Text(sans, x0+8, y0+7, "SECTOR "+lv.ID, pixel.Copy)
	// The conduits in play, as five squares.
	for r := 0; r < Rows; r++ {
		in := false
		for _, l := range lv.Lanes {
			in = in || l == r
		}
		sx, sy := x0+panelW-8-(Rows-r)*12, y0+7
		cv.Rect(sx, sy, 9, 9, false)
		if in {
			cv.Rect(sx, sy, 9, 9, true)
		} else {
			outline(cv, sx, sy, 9, 9)
		}
	}
	// What comes, then what the player has, and what the sector gives.
	for n, k := range lv.malware() {
		cv.Draw(sheet.Image(map[enemyKind]string{Drone: "drone", Worm: "worm", Trojan: "trojan"}[k]), x0+8+n*30, y0+26, pixel.Copy)
	}
	for k := Kind(0); k < kinds; k++ {
		px, py := x0+8+int(k)*30, y0+62
		if g.prog.unlocked(k) {
			cv.Draw(sheet.Image(programSprites[k]), px, py, pixel.Copy)
		} else {
			cv.Draw(sheet.Image("lock"), px+6, py+6, pixel.Copy)
		}
	}
	if lv.Reward != rewardNone {
		tiny := pixel.FontNamed("tiny3x5")
		rx := x0 + panelW - 8 - spriteSize
		cv.Text(tiny, rx-8, y0+70, "+", pixel.Copy)
		cv.Draw(sheet.Image(programSprites[lv.Reward]), rx, y0+62, pixel.Copy)
		if g.prog.unlocked(lv.Reward) {
			cv.Dither(rx, y0+62, spriteSize, spriteSize, 0.4, false) // already the player's
		}
	}
}

// drawReward draws the program a sector gave: a box with it, its name and what
// it does.
func drawReward(g *game, cv *pixel.Canvas) {
	sans, tiny := pixel.FontNamed("sans5x7"), pixel.FontNamed("tiny3x5")
	k := g.reward
	w, h := 168, 112
	x, y := (games.Width-w)/2, games.HUDHeight+(games.AreaHeight-h)/2
	cv.Dither(x-3, y-3, w+8, h+8, 0.5, true)
	cv.Rect(x-4, y-4, w+8, h+8, false)
	cv.Frame(x-4, y-4, w+8, h+8, 2)
	cv.Draw(sheet.Image(programSprites[k]), x+(w-spriteSize)/2, y+10, pixel.Copy)
	name := programs[k].name
	cv.Text(sans, x+(w-sans.Width(name))/2, y+44, name, pixel.Copy)
	tip := cardTips[k]
	cv.Text(tiny, x+(w-tiny.Width(tip[1]))/2, y+60, tip[1], pixel.Copy)
	cost := itoa(k.Cost()) + string(wattSign)
	cv.Draw(sheet.Image(roleSprites[k]), x+w/2-tiny.Width(cost)/2-12, y+78, pixel.Copy)
	cv.Text(tiny, x+w/2-tiny.Width(cost)/2+4, y+80, cost, pixel.Copy)
	// A shine round the program.
	cx0, cy0 := x+w/2, y+10+spriteSize/2
	for _, p := range [][2]int{{-22, -6}, {22, -8}, {-16, 14}, {18, 16}} {
		cv.Rect(cx0+p[0]-2, cy0+p[1], 5, 1, true)
		cv.Rect(cx0+p[0], cy0+p[1]-2, 1, 5, true)
	}
}

// refusedShade is how much a program being placed is dimmed where it cannot go.
const refusedShade = 0.75

// closedShade is how much of a place that is not the player's yet is shaded: one
// pixel in sixteen, so that it is there without calling for the eye.
const closedShade = 0.0625

// dashed draws a ring of dots, one pixel in two, round a rectangle: a place that is
// there, and not yet filled.
func dashed(cv *pixel.Canvas, x, y, w, h int) {
	for i := 0; i < w; i += 2 {
		cv.Set(x+i, y, true)
		cv.Set(x+i, y+h-1, true)
	}
	for j := 0; j < h; j += 2 {
		cv.Set(x, y+j, true)
		cv.Set(x+w-1, y+j, true)
	}
}
