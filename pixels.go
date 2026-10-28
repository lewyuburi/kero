package main

import (
	"image"
	"image/color"

	"github.com/egoist/mygo/ui"
)

// pixelCanvas is the screen of a game in the style of a one-bit handheld:
// every pixel is ink or paper, drawn into a small picture that the pane
// shows enlarged, its pixels as crisp squares. Shades come from dithering
// (Dither), not from gray.
type pixelCanvas struct {
	w, h int
	ink  []bool
	img  *image.RGBA
	bm   *ui.Bitmap
}

func newPixelCanvas(w, h int) *pixelCanvas {
	return &pixelCanvas{w: w, h: h, ink: make([]bool, w*h)}
}

// Clear makes every pixel paper.
func (cv *pixelCanvas) Clear() { clear(cv.ink) }

// Set makes a pixel ink or paper, if it is on the canvas.
func (cv *pixelCanvas) Set(x, y int, ink bool) {
	if x >= 0 && y >= 0 && x < cv.w && y < cv.h {
		cv.ink[y*cv.w+x] = ink
	}
}

// Rect fills a rectangle with ink or paper.
func (cv *pixelCanvas) Rect(x, y, w, h int, ink bool) {
	for j := y; j < y+h; j++ {
		for i := x; i < x+w; i++ {
			cv.Set(i, j, ink)
		}
	}
}

// bayer is the 4×4 matrix that orders pixels as a shade fills in.
var bayer = [4][4]int{{0, 8, 2, 10}, {12, 4, 14, 6}, {3, 11, 1, 9}, {15, 7, 13, 5}}

// Dither fills a rectangle with a shade of ink or of paper (as ink says),
// 0 none to 1 solid, in the pattern that a shade always has, so that the
// same shade lines up wherever it is drawn.
func (cv *pixelCanvas) Dither(x, y, w, h int, level float32, ink bool) {
	for j := y; j < y+h; j++ {
		for i := x; i < x+w; i++ {
			if float32(bayer[j&3][i&3]) < level*16 {
				cv.Set(i, j, ink)
			}
		}
	}
}

// Frame draws an outline of a given thickness, with its corners chipped.
func (cv *pixelCanvas) Frame(x, y, w, h, thick int) {
	cv.Rect(x, y, w, h, true)
	cv.Rect(x+thick, y+thick, w-2*thick, h-2*thick, false)
	for _, c := range [][2]int{{x, y}, {x + w - 1, y}, {x, y + h - 1}, {x + w - 1, y + h - 1}} {
		cv.Set(c[0], c[1], false)
	}
}

// Sprite draws rows of characters: # is ink, . is paper, anything else
// leaves what is there.
func (cv *pixelCanvas) Sprite(x, y int, rows ...string) {
	for j, row := range rows {
		for i, r := range row {
			switch r {
			case '#':
				cv.Set(x+i, y+j, true)
			case '.':
				cv.Set(x+i, y+j, false)
			}
		}
	}
}

// Bitmap draws the canvas in the colors given and returns the picture to
// show, which is the same one each time, updated.
func (cv *pixelCanvas) Bitmap(ink, paper color.RGBA) *ui.Bitmap {
	if cv.img == nil {
		cv.img = image.NewRGBA(image.Rect(0, 0, cv.w, cv.h))
	}
	for i, on := range cv.ink {
		c := paper
		if on {
			c = ink
		}
		o := i * 4
		cv.img.Pix[o], cv.img.Pix[o+1], cv.img.Pix[o+2], cv.img.Pix[o+3] = c.R, c.G, c.B, c.A
	}
	if cv.bm == nil {
		cv.bm = ui.NewBitmap(cv.img).Pixelated()
	} else {
		cv.bm.Update(cv.img)
	}
	return cv.bm
}

// screenColors are the ink and the paper of a game's screen: dark on a
// pale, greenish gray, as a reflective display is; the other way about in
// the dark.
func screenColors(dark bool) (ink, paper color.RGBA) {
	ink, paper = color.RGBA{0x2f, 0x2d, 0x27, 0xff}, color.RGBA{0xb5, 0xb3, 0xa6, 0xff}
	if dark {
		ink, paper = color.RGBA{0xc9, 0xc7, 0xb9, 0xff}, color.RGBA{0x24, 0x23, 0x1f, 0xff}
	}
	return ink, paper
}
