package snake

import (
	_ "embed"
	"time"

	"kero/internal/games"
	"kero/internal/pixel"
)

func init() {
	games.Register(games.Kind{
		ID: "snake", Title: "Snake",
		New:  func() games.Game { return New(time.Now().UnixNano()) },
		Draw: draw,
		Icon: sheet.Image("icon"),
	})
}

// The snake's screen, drawn in a one-bit canvas: cells of Cell pixels in a
// framed board that casts a dithered shadow. The snake is slim: Cell-6 pixels in
// its cell, three of room each side, and its body is a band of that width, its
// segments joined; what it eats fills the cell. The board is
// 19 by 13 cells of 16 pixels, 304 by 208, in the middle of the area the game
// has under the band of the score and the hint, with 4 pixels for its frame at
// each side and its shadow below and to the right. The cells are large enough
// to draw what is in them.
const (
	Cell = 16
	Cols = 19
	Rows = 13
	// What is in a cell, and the room round it.
	pad    = 3
	sprite = Cell - 2*pad
	// Where the board's top left is on the screen.
	origin = (games.Width - Cols*Cell) / 2
	top    = games.HUDHeight + (games.AreaHeight-Rows*Cell)/2
)

//go:embed snake.sheet
var sheetSource string

var sheet = pixel.MustSheet(sheetSource)

func draw(g games.Game, cv *pixel.Canvas) {
	sn := g.(*snake)
	bw, bh := sn.w*Cell, sn.h*Cell
	ox, oy := origin, top
	cv.Clear()
	// The board, with its shadow, and a dot at the center of each cell.
	cv.Dither(ox-3, oy-3, bw+10, bh+10, 0.5, true)
	cv.Rect(ox-4, oy-4, bw+8, bh+8, false)
	cv.Frame(ox-4, oy-4, bw+8, bh+8, 2)
	for y := 0; y < sn.h; y++ {
		for x := 0; x < sn.w; x++ {
			cv.Set(ox+x*Cell+Cell/2-1, oy+y*Cell+Cell/2-1, true)
		}
	}
	// What is in a cell is a pixel in from its edges.
	cell := func(p point) (int, int) { return ox + p.x*Cell + pad, oy + p.y*Cell + pad }
	// What it eats fills its cell: it is not the snake, which has room round it.
	whole := func(p point) (int, int) { return ox + p.x*Cell, oy + p.y*Cell }
	fx, fy := whole(sn.food)
	cv.Draw(sheet.Image("apple"), fx, fy, pixel.Copy)
	// A bonus that is about to go blinks.
	if b := sn.bonus; b != nil && (b.ttl > 12 || b.ttl%2 == 1) {
		bx, by := whole(b.at)
		cv.Draw(sheet.Image(map[bonusKind]string{bonusCherry: "cherry", bonusStar: "star", bonusClock: "clock"}[b.kind]), bx, by, pixel.Copy)
	}
	n := len(sn.body)
	for i := n - 1; i >= 0; i-- {
		x, y := cell(sn.body[i])
		switch {
		case i == 0:
			cv.Draw(sheet.Image("head-"+direction(sn.dir)), x, y, pixel.Copy)
		case i == n-1 && n > 3:
			// The tail narrows, away from the body.
			prev := sn.body[i-1]
			cv.Draw(sheet.Image("tail-"+direction(point{sn.body[i].x - prev.x, sn.body[i].y - prev.y})), x, y, pixel.Copy)
		case i%2 == 0:
			cv.Draw(sheet.Image("body2"), x, y, pixel.Copy)
		default:
			cv.Draw(sheet.Image("body"), x, y, pixel.Copy)
		}
		if i > 0 {
			// A band between this segment and the one before it, to join them.
			px, py := cell(sn.body[i-1])
			switch {
			case px == x:
				join(cv, x, min(y, py)+sprite, sprite, pad*2, false)
			case py == y:
				join(cv, min(x, px)+sprite, y, pad*2, sprite, true)
			}
		}
	}
	if sn.over {
		// The screen washes out.
		cv.Dither(ox, oy, bw, bh, 0.55, false)
	}
	// A bonus taken flashes the board, twice; dying shakes the screen.
	if sn.flash > 0 && sn.flash%2 == 0 {
		cv.Invert(ox, oy, bw, bh)
	}
	cv.SetShift(pixel.ShakeOffset(sn.shake, 1))
}

// direction names a step of the snake, for the sprites that look that way.
func direction(d point) string {
	switch d {
	case point{-1, 0}:
		return "left"
	case point{0, -1}:
		return "up"
	case point{0, 1}:
		return "down"
	}
	return "right"
}

// join draws the band between two segments as a piece of the body: its sides are
// ink, as the segments' borders are, and it has the light texture of the body
// inside, so that the body is one tube. across says the band runs left to right, between segments side by
// side.
func join(cv *pixel.Canvas, x, y, w, h int, across bool) {
	cv.Rect(x, y, w, h, false)
	if across {
		cv.Rect(x, y, w, 1, true)
		cv.Rect(x, y+h-1, w, 1, true)
		cv.Dither(x, y+1, w, h-2, bodyShade, true)
		return
	}
	cv.Rect(x, y, 1, h, true)
	cv.Rect(x+w-1, y, 1, h, true)
	cv.Dither(x+1, y, w-2, h, bodyShade, true)
}

// bodyShade is how much of the inside of the body is ink: one pixel in four.
const bodyShade = 0.25
