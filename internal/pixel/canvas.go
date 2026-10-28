// Package pixel draws the screens of games in the style of a one-bit
// handheld: every pixel is ink or paper, drawn into a small canvas that a
// window shows enlarged, its pixels as crisp squares. Shades come from
// dithering, not from gray. It has no window of its own: a Canvas renders to
// an image, which the app puts on the screen.
package pixel

import (
	"image"
	"image/color"
)

// A Canvas is a picture of ink and paper.
type Canvas struct {
	W, H           int
	ink            []bool
	shiftX, shiftY int
}

// NewCanvas makes a canvas of w by h pixels, all paper.
func NewCanvas(w, h int) *Canvas {
	return &Canvas{W: w, H: h, ink: make([]bool, w*h)}
}

// Clear makes every pixel paper, and puts the canvas back where it was
// shifted from (SetShift).
func (cv *Canvas) Clear() {
	clear(cv.ink)
	cv.shiftX, cv.shiftY = 0, 0
}

// Set makes a pixel ink or paper, if it is on the canvas.
func (cv *Canvas) Set(x, y int, ink bool) {
	if x >= 0 && y >= 0 && x < cv.W && y < cv.H {
		cv.ink[y*cv.W+x] = ink
	}
}

// Get tells whether a pixel is ink; one off the canvas is paper.
func (cv *Canvas) Get(x, y int) bool {
	return x >= 0 && y >= 0 && x < cv.W && y < cv.H && cv.ink[y*cv.W+x]
}

// Rect fills a rectangle with ink or paper. Shapes are drawn as they say,
// whatever the mode of an image: modes are for images and text.
func (cv *Canvas) Rect(x, y, w, h int, ink bool) {
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
// same shade lines up wherever it is drawn. A shade is a checkerboard from
// the half up, which flashes if it moves a pixel at a time: shift a canvas
// by whole pairs of pixels (ShakeOffset does), or dither after moving.
func (cv *Canvas) Dither(x, y, w, h int, level float32, ink bool) {
	for j := y; j < y+h; j++ {
		for i := x; i < x+w; i++ {
			if float32(bayer[j&3][i&3]) < level*16 {
				cv.Set(i, j, ink)
			}
		}
	}
}

// Frame draws an outline of a given thickness, with its corners chipped.
func (cv *Canvas) Frame(x, y, w, h, thick int) {
	cv.Rect(x, y, w, h, true)
	cv.Rect(x+thick, y+thick, w-2*thick, h-2*thick, false)
	for _, c := range [][2]int{{x, y}, {x + w - 1, y}, {x, y + h - 1}, {x + w - 1, y + h - 1}} {
		cv.Set(c[0], c[1], false)
	}
}

// Invert turns the ink in a rectangle to paper and the paper to ink, as a
// flash does.
func (cv *Canvas) Invert(x, y, w, h int) {
	for j := y; j < y+h; j++ {
		for i := x; i < x+w; i++ {
			if i >= 0 && j >= 0 && i < cv.W && j < cv.H {
				cv.ink[j*cv.W+i] = !cv.ink[j*cv.W+i]
			}
		}
	}
}

// SetShift moves what the canvas shows by dx, dy pixels, as a shake does,
// until the next Clear. Move by whole pairs of pixels where there is
// dither, or it flashes.
func (cv *Canvas) SetShift(dx, dy int) { cv.shiftX, cv.shiftY = dx, dy }

// ShakeOffset is where a shake of the given strength has the screen at a
// frame of it: it goes down as the frames left do, to nothing, and moves by
// pairs of pixels. A frame count of 0 is still.
func ShakeOffset(framesLeft, amp int) (dx, dy int) {
	if framesLeft <= 0 || amp <= 0 {
		return 0, 0
	}
	a := 2 * min(amp, (framesLeft+3)/4)
	if framesLeft%2 == 0 {
		return a, 0
	}
	return -a, 0
}

// RGBA draws the canvas in the colors given, into dst if it is a picture of
// its size, else into a new one, and returns it.
func (cv *Canvas) RGBA(ink, paper color.RGBA, dst *image.RGBA) *image.RGBA {
	if dst == nil || dst.Bounds().Dx() != cv.W || dst.Bounds().Dy() != cv.H {
		dst = image.NewRGBA(image.Rect(0, 0, cv.W, cv.H))
	}
	for y := 0; y < cv.H; y++ {
		for x := 0; x < cv.W; x++ {
			c := paper
			if cv.Get(x-cv.shiftX, y-cv.shiftY) {
				c = ink
			}
			o := (y*cv.W + x) * 4
			dst.Pix[o], dst.Pix[o+1], dst.Pix[o+2], dst.Pix[o+3] = c.R, c.G, c.B, c.A
		}
	}
	return dst
}
