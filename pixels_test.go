package main

import "testing"

func TestPixelCanvasDrawsInInkAndPaper(t *testing.T) {
	cv := newPixelCanvas(8, 8)
	cv.Rect(1, 1, 3, 3, true)
	cv.Set(2, 2, false)
	cv.Set(-1, 0, true) // off the canvas: nothing
	cv.Set(8, 8, true)
	var n int
	for _, on := range cv.ink {
		if on {
			n++
		}
	}
	if n != 8 {
		t.Errorf("%d pixels inked, want the square's 9 less the hole", n)
	}
	cv.Clear()
	cv.Sprite(0, 0, "#.#", " #.")
	if !cv.ink[0] || cv.ink[1] || !cv.ink[2] || cv.ink[8] || !cv.ink[9] {
		t.Errorf("sprite drawn as %v", cv.ink[:10])
	}
}

// A shade fills in the same pattern wherever it is drawn, so that two
// rectangles of one shade meet without a seam, and a darker one only adds.
func TestDitherKeepsOnePattern(t *testing.T) {
	count := func(level float32) (n int) {
		cv := newPixelCanvas(16, 16)
		cv.Dither(0, 0, 16, 16, level, true)
		for _, on := range cv.ink {
			if on {
				n++
			}
		}
		return n
	}
	if count(0) != 0 || count(1) != 256 || count(0.5) != 128 {
		t.Errorf("shades 0, 0.5, 1 ink %d, %d, %d of 256 pixels", count(0), count(0.5), count(1))
	}
	a, b := newPixelCanvas(16, 16), newPixelCanvas(16, 16)
	a.Dither(0, 0, 16, 16, 0.5, true)
	b.Dither(0, 0, 8, 16, 0.5, true)
	b.Dither(8, 0, 8, 16, 0.5, true)
	for i := range a.ink {
		if a.ink[i] != b.ink[i] {
			t.Fatalf("pixel %d differs between one rectangle and two", i)
		}
	}
	prev := count(0)
	for l := float32(0.1); l <= 1; l += 0.1 {
		if n := count(l); n < prev {
			t.Errorf("shade %.1f inks %d, fewer than the one before's %d", l, n, prev)
		}
		prev = count(l)
	}
}

// Every glyph is 5 by 7, and the font has what a score and a hint say.
func TestPixelFontIsWhole(t *testing.T) {
	for r, g := range pixelGlyphs {
		for j, row := range g {
			if len(row) != glyphW {
				t.Errorf("glyph %q row %d is %q: not %d wide", r, j, row, glyphW)
			}
		}
	}
	for _, r := range "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789.,-:!?/+·" {
		if _, ok := pixelGlyphs[r]; !ok {
			t.Errorf("no glyph for %q", r)
		}
	}
	cv := newPixelCanvas(40, 10)
	end := cv.Text(1, 1, "Hi 7")
	if want := 1 + cv.TextWidth("Hi 7"); end != want || cv.TextWidth("Hi 7") != 4*glyphAdvance-1 {
		t.Errorf("text ends at %d, width %d, want %d", end, cv.TextWidth("Hi 7"), want)
	}
	// A lower case letter is its capital's, and a rune the font lacks is a question mark.
	a, b := newPixelCanvas(8, 8), newPixelCanvas(8, 8)
	a.Text(0, 0, "h")
	b.Text(0, 0, "H")
	for i := range a.ink {
		if a.ink[i] != b.ink[i] {
			t.Fatal("h is not drawn as H")
		}
	}
	c, d := newPixelCanvas(8, 8), newPixelCanvas(8, 8)
	c.Text(0, 0, "é")
	d.Text(0, 0, "?")
	for i := range c.ink {
		if c.ink[i] != d.ink[i] {
			t.Fatal("a rune the font lacks is not a question mark")
		}
	}
}
