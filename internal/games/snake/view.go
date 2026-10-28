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
	})
}

// The snake's screen, drawn in a one-bit canvas: cells of Cell pixels in a
// framed board that casts a dithered shadow. The board is 30 by 20 cells of 8
// pixels, 240 by 160, in the middle of the area the game has under the band
// of the score and the hint, with 4 pixels for its frame at each side and its
// shadow below and to the right.
const (
	Cell = 8
	Cols = 30
	Rows = 20
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
			cv.Set(ox+x*Cell+3, oy+y*Cell+3, true)
		}
	}
	cell := func(p point) (int, int) { return ox + p.x*Cell, oy + p.y*Cell }
	fx, fy := cell(sn.food)
	cv.Draw(sheet.Image("apple"), fx, fy, pixel.Copy)
	// A bonus that is about to go blinks.
	if b := sn.bonus; b != nil && (b.ttl > 12 || b.ttl%2 == 1) {
		bx, by := cell(b.at)
		cv.Draw(sheet.Image(map[bonusKind]string{bonusCherry: "cherry", bonusStar: "star", bonusClock: "clock"}[b.kind]), bx, by, pixel.Copy)
	}
	n := len(sn.body)
	for i := n - 1; i >= 0; i-- {
		x, y := cell(sn.body[i])
		switch {
		case i == 0:
			cv.Draw(sheet.Image("body"), x, y, pixel.Copy)
			// Eyes, toward where it goes.
			eyes := map[point][][2]int{
				{1, 0}:  {{5, 2}, {5, 5}},
				{-1, 0}: {{2, 2}, {2, 5}},
				{0, -1}: {{2, 2}, {5, 2}},
				{0, 1}:  {{2, 5}, {5, 5}},
			}[sn.dir]
			for _, e := range eyes {
				cv.Set(x+e[0], y+e[1], false)
			}
		case i == n-1 && n > 3:
			cv.Draw(sheet.Image("tail"), x, y, pixel.Copy)
		default:
			cv.Draw(sheet.Image("body"), x, y, pixel.Copy)
			// Scales: a paper dot on every other segment.
			if i%2 == 0 {
				cv.Rect(x+3, y+3, 2, 2, false)
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
