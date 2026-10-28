package pixel

import (
	"fmt"
	"strconv"
	"strings"
)

// A Sheet is the sprites of a game, by name, and the animations that run
// through them. Its file is text, so that a diff shows what changed in a
// sprite: a sprite is a line "sprite name" and its rows, # ink, . paper and
// a space transparent; an animation is one line, "anim name ticks frame...",
// the ticks that each frame stays and the sprites it goes through. A line
// that starts with // is a comment.
//
//	sprite apple
//	....#...
//	.######.
//	...
//
//	anim wiggle 6 apple apple2
type Sheet struct {
	images map[string]*Image
	anims  map[string]*Anim
}

// An Anim is frames, each shown for some ticks, and then the first again.
type Anim struct {
	Frames []*Image
	Ticks  int
}

// At is the frame shown at a tick.
func (a *Anim) At(tick int) *Image {
	if len(a.Frames) == 0 {
		return nil
	}
	return a.Frames[(max(tick, 0)/max(a.Ticks, 1))%len(a.Frames)]
}

// ParseSheet reads a sheet.
func ParseSheet(src string) (*Sheet, error) {
	s := &Sheet{images: map[string]*Image{}, anims: map[string]*Anim{}}
	var name string
	var rows []string
	flush := func() {
		if name != "" {
			s.images[name] = ParseImage(rows, true)
		}
		name, rows = "", nil
	}
	for _, line := range strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n") {
		switch {
		case strings.HasPrefix(line, "//"):
		case strings.HasPrefix(line, "sprite "):
			flush()
			name = strings.TrimSpace(strings.TrimPrefix(line, "sprite "))
			if name == "" {
				return nil, fmt.Errorf("sheet: a sprite with no name")
			}
		case strings.HasPrefix(line, "anim "):
			flush()
			f := strings.Fields(line)
			if len(f) < 4 {
				return nil, fmt.Errorf("sheet: %q needs a name, ticks and frames", line)
			}
			ticks, err := strconv.Atoi(f[2])
			if err != nil || ticks < 1 {
				return nil, fmt.Errorf("sheet: %q: ticks are a whole number from 1", line)
			}
			a := &Anim{Ticks: ticks}
			for _, fr := range f[3:] {
				img := s.images[fr]
				if img == nil {
					return nil, fmt.Errorf("sheet: animation %s goes through %s, which comes later or is not there", f[1], fr)
				}
				a.Frames = append(a.Frames, img)
			}
			s.anims[f[1]] = a
		case line == "":
			flush()
		case name != "":
			rows = append(rows, line)
		}
	}
	flush()
	return s, nil
}

// Image is the sprite of that name, nil if there is none, which draws as
// nothing.
func (s *Sheet) Image(name string) *Image { return s.images[name] }

// Anim is the animation of that name, or one of no frames.
func (s *Sheet) Anim(name string) *Anim {
	if a := s.anims[name]; a != nil {
		return a
	}
	return &Anim{Ticks: 1}
}

// MustSheet is ParseSheet for what is part of the program: a mistake in it
// is the program's.
func MustSheet(src string) *Sheet {
	s, err := ParseSheet(src)
	if err != nil {
		panic("pixel: " + err.Error())
	}
	return s
}

// A Tilemap is a grid of sprites by name, which a board is.
type Tilemap struct {
	Cols, Rows   int
	TileW, TileH int
	tiles        []string
}

// NewTilemap makes a grid of cols by rows tiles of w by h pixels, all empty.
func NewTilemap(cols, rows, w, h int) *Tilemap {
	return &Tilemap{Cols: cols, Rows: rows, TileW: w, TileH: h, tiles: make([]string, cols*rows)}
}

// Set puts the sprite of that name in a cell; "" empties it.
func (t *Tilemap) Set(col, row int, name string) {
	if col >= 0 && row >= 0 && col < t.Cols && row < t.Rows {
		t.tiles[row*t.Cols+col] = name
	}
}

// At is the name of the sprite in a cell.
func (t *Tilemap) At(col, row int) string {
	if col < 0 || row < 0 || col >= t.Cols || row >= t.Rows {
		return ""
	}
	return t.tiles[row*t.Cols+col]
}

// Draw draws the tiles with the map's top left at x, y.
func (t *Tilemap) Draw(cv *Canvas, sheet *Sheet, x, y int, m Mode) {
	for r := 0; r < t.Rows; r++ {
		for c := 0; c < t.Cols; c++ {
			if name := t.tiles[r*t.Cols+c]; name != "" {
				cv.Draw(sheet.Image(name), x+c*t.TileW, y+r*t.TileH, m)
			}
		}
	}
}
