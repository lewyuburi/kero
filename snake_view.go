package main

// The snake's screen, drawn in a one-bit canvas (pixels.go): cells of
// snakeCell pixels in a framed board that casts a dithered shadow.
//
// The board is 5 to 3, as the screen of a handheld is: 25 by 15 cells of 8
// pixels is 200 by 120, half of Playdate's 400 by 240. Its size never
// follows the pane's; the pane gives it the room it has, in proportion.
const (
	snakeCell = 8
	snakeCols = 25
	snakeRows = 15
	// The board starts snakeOrigin pixels in, which leaves its frame a
	// pixel from the top and the left, where nothing needs room; the shadow
	// the frame casts takes what is left at the right and the bottom.
	snakeOrigin = 5
	snakeEdge   = 10
	// Under the band of the score and the hint (hudHeight), with the same
	// room the frame has at the left.
	snakeTop = hudHeight + 4
)

func snakeCanvas() (w, h int) {
	return snakeOrigin + snakeCols*snakeCell + snakeEdge, snakeTop + snakeRows*snakeCell + snakeEdge
}

var (
	snakeBody = []string{
		".######.",
		"########",
		"########",
		"########",
		"########",
		"########",
		"########",
		".######.",
	}
	snakeTail = []string{
		"........",
		"..####..",
		".######.",
		".######.",
		".######.",
		".######.",
		"..####..",
		"........",
	}
	// The bonuses: a cherry, a star and a clock.
	snakeBonus = map[bonusKind][]string{
		bonusCherry: {
			".....##.",
			"....#.#.",
			"...#..#.",
			".##..###",
			"####.###",
			"####.###",
			".##...#.",
			"........",
		},
		bonusStar: {
			"...##...",
			"...##...",
			"########",
			".######.",
			"..####..",
			".######.",
			".##..##.",
			".#....#.",
		},
		bonusClock: {
			".######.",
			".#....#.",
			"..#..#..",
			"...##...",
			"...##...",
			"..#..#..",
			".#.##.#.",
			".######.",
		},
	}
	snakeApple = []string{
		"....#...",
		"...##.#.",
		".######.",
		"#.#.#.##",
		"##.#.#.#",
		"#.#.#.##",
		".######.",
		"..####..",
	}
)

func drawSnake(g game, cv *pixelCanvas) {
	sn := g.(*snake)
	bw, bh := sn.w*snakeCell, sn.h*snakeCell
	ox, oy := snakeOrigin, snakeTop
	cv.Clear()
	// The board, with its shadow, and a dot at the center of each cell.
	cv.Dither(ox-3, oy-3, bw+10, bh+10, 0.5, true)
	cv.Rect(ox-4, oy-4, bw+8, bh+8, false)
	cv.Frame(ox-4, oy-4, bw+8, bh+8, 2)
	for y := 0; y < sn.h; y++ {
		for x := 0; x < sn.w; x++ {
			cv.Set(ox+x*snakeCell+3, oy+y*snakeCell+3, true)
		}
	}
	cell := func(p point) (int, int) { return ox + p.x*snakeCell, oy + p.y*snakeCell }
	fx, fy := cell(sn.food)
	cv.Rect(fx, fy, snakeCell, snakeCell, false)
	cv.Sprite(fx, fy, snakeApple...)
	// A bonus that is about to go blinks.
	if b := sn.bonus; b != nil && (b.ttl > 12 || b.ttl%2 == 1) {
		bx, by := cell(b.at)
		cv.Rect(bx, by, snakeCell, snakeCell, false)
		cv.Sprite(bx, by, snakeBonus[b.kind]...)
	}
	n := len(sn.body)
	for i := n - 1; i >= 0; i-- {
		x, y := cell(sn.body[i])
		cv.Rect(x, y, snakeCell, snakeCell, false)
		switch {
		case i == 0:
			cv.Sprite(x, y, snakeBody...)
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
			cv.Sprite(x, y, snakeTail...)
		default:
			cv.Sprite(x, y, snakeBody...)
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
}
