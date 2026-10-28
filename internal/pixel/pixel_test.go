package pixel

import (
	"strings"
	"testing"
)

func count(cv *Canvas) (n int) {
	for y := 0; y < cv.H; y++ {
		for x := 0; x < cv.W; x++ {
			if cv.Get(x, y) {
				n++
			}
		}
	}
	return n
}

func TestCanvasDrawsInInkAndPaper(t *testing.T) {
	cv := NewCanvas(8, 8)
	cv.Rect(1, 1, 3, 3, true)
	cv.Set(2, 2, false)
	cv.Set(-1, 0, true) // off the canvas: nothing
	cv.Set(8, 8, true)
	if n := count(cv); n != 8 {
		t.Errorf("%d pixels inked, want the square's 9 less the hole", n)
	}
	if cv.Get(-1, 0) || cv.Get(8, 8) {
		t.Error("a pixel off the canvas is ink")
	}
	cv.Invert(0, 0, 4, 4)
	if cv.Get(1, 1) || !cv.Get(2, 2) || !cv.Get(0, 0) {
		t.Error("invert did not turn ink and paper over")
	}
}

// A shade fills in the same pattern wherever it is drawn, so that two
// rectangles of one shade meet without a seam, and a darker one only adds.
func TestDitherKeepsOnePattern(t *testing.T) {
	shade := func(level float32) int {
		cv := NewCanvas(16, 16)
		cv.Dither(0, 0, 16, 16, level, true)
		return count(cv)
	}
	if shade(0) != 0 || shade(1) != 256 || shade(0.5) != 128 {
		t.Errorf("shades 0, 0.5, 1 ink %d, %d, %d of 256 pixels", shade(0), shade(0.5), shade(1))
	}
	a, b := NewCanvas(16, 16), NewCanvas(16, 16)
	a.Dither(0, 0, 16, 16, 0.5, true)
	b.Dither(0, 0, 8, 16, 0.5, true)
	b.Dither(8, 0, 8, 16, 0.5, true)
	for i := range a.ink {
		if a.ink[i] != b.ink[i] {
			t.Fatalf("pixel %d differs between one rectangle and two", i)
		}
	}
	prev := shade(0)
	for l := float32(0.1); l <= 1; l += 0.1 {
		if n := shade(l); n < prev {
			t.Errorf("shade %.1f inks %d, fewer than the one before's %d", l, n, prev)
		} else {
			prev = n
		}
	}
}

// What a mode does, on a pixel of ink and of paper over ink and over paper,
// and on a pixel that is transparent, which nothing does anything to.
func TestDrawModes(t *testing.T) {
	img := ParseImage([]string{"#. "}, true) // ink, paper, transparent
	for _, tc := range []struct {
		mode        Mode
		over        [2]bool // what is under the first two pixels
		want        [2]bool
		keepsThird  bool
		description string
	}{
		{Copy, [2]bool{false, true}, [2]bool{true, false}, true, "copy"},
		{InkOnly, [2]bool{false, true}, [2]bool{true, true}, true, "ink only: paper is transparent"},
		{PaperOnly, [2]bool{false, true}, [2]bool{false, false}, true, "paper only: ink is transparent"},
		{FillInk, [2]bool{false, false}, [2]bool{true, true}, true, "fill ink: a stencil"},
		{FillPaper, [2]bool{true, true}, [2]bool{false, false}, true, "fill paper"},
		{XOR, [2]bool{false, true}, [2]bool{true, true}, true, "xor: ink turns over"},
		{NXOR, [2]bool{false, true}, [2]bool{false, false}, true, "nxor: paper turns over"},
		{Inverted, [2]bool{true, false}, [2]bool{false, true}, true, "inverted"},
	} {
		cv := NewCanvas(3, 1)
		cv.Set(0, 0, tc.over[0])
		cv.Set(1, 0, tc.over[1])
		cv.Set(2, 0, true)
		cv.Draw(img, 0, 0, tc.mode)
		if cv.Get(0, 0) != tc.want[0] || cv.Get(1, 0) != tc.want[1] || cv.Get(2, 0) != tc.keepsThird {
			t.Errorf("%s: got %v %v %v, want %v %v %v", tc.description, cv.Get(0, 0), cv.Get(1, 0), cv.Get(2, 0), tc.want[0], tc.want[1], tc.keepsThird)
		}
	}
	// Drawing nothing is no harm.
	NewCanvas(2, 2).Draw(nil, 0, 0, Copy)
}

func TestImageRoundTrips(t *testing.T) {
	rows := []string{"#.#", " # ", "..."}
	got := ParseImage(rows, true).Rows()
	for i := range rows {
		if got[i] != rows[i] {
			t.Errorf("row %d is %q, was %q", i, got[i], rows[i])
		}
	}
	// A font's paper is transparent.
	if r := ParseImage([]string{"#.#"}, false).Rows()[0]; r != "# #" {
		t.Errorf("paper not opaque: %q", r)
	}
}

func TestShakeGoesByPairsOfPixelsAndDown(t *testing.T) {
	if dx, dy := ShakeOffset(0, 1); dx != 0 || dy != 0 {
		t.Errorf("a shake with no frames moves %d,%d", dx, dy)
	}
	last := 99
	for f := 12; f >= 1; f-- {
		dx, _ := ShakeOffset(f, 1)
		if dx%2 != 0 {
			t.Errorf("frame %d moves %d: not a pair of pixels", f, dx)
		}
		if a := max(dx, -dx); a > last {
			t.Errorf("frame %d moves %d, more than the frame before", f, a)
		} else {
			last = a
		}
	}
	cv := NewCanvas(4, 1)
	cv.Set(0, 0, true)
	cv.SetShift(2, 0)
	img := cv.RGBA(Themes[0].Ink, Themes[0].Paper, nil)
	if img.RGBAAt(2, 0) != Themes[0].Ink || img.RGBAAt(0, 0) != Themes[0].Paper {
		t.Error("the shift did not move what is shown")
	}
	cv.Clear()
	if cv.shiftX != 0 {
		t.Error("clear left the shift")
	}
}

func TestFontsParseAndMeasure(t *testing.T) {
	for _, name := range []string{"sans5x7", "tiny3x5"} {
		f := FontNamed(name)
		if f.Name != name || f.Height == 0 {
			t.Fatalf("font %s read as %q, %d tall", name, f.Name, f.Height)
		}
		for r, g := range f.glyphs {
			if g.H != f.Height {
				t.Errorf("%s: glyph %q is %d tall, not %d", name, r, g.H, f.Height)
			}
		}
		for _, r := range "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789.:-!?/" {
			if f.glyphs[r] == nil {
				t.Errorf("%s has no glyph for %q", name, r)
			}
		}
	}
	f := FontNamed("sans5x7")
	// Glyphs have widths of their own: an I is narrower than an M.
	if f.glyphs['I'].W >= f.glyphs['M'].W {
		t.Error("I is not narrower than M")
	}
	// The width has the tracking between glyphs and none after the last.
	if got, want := f.Width("IM"), f.glyphs['I'].W+f.Tracking+f.glyphs['M'].W; got != want {
		t.Errorf("IM is %d wide, want %d", got, want)
	}
	if f.Width("") != 0 {
		t.Error("nothing has a width")
	}
	// A pair that kerns is closer.
	if f.Width("AV") != f.glyphs['A'].W+f.Tracking-1+f.glyphs['V'].W {
		t.Errorf("AV is %d wide, not kerned", f.Width("AV"))
	}
	// A small letter is the capital, and a rune the font lacks a question mark.
	a, b := NewCanvas(10, 8), NewCanvas(10, 8)
	a.Text(f, 0, 0, "h", Copy)
	b.Text(f, 0, 0, "H", Copy)
	for i := range a.ink {
		if a.ink[i] != b.ink[i] {
			t.Fatal("h is not drawn as H")
		}
	}
	c, d := NewCanvas(10, 8), NewCanvas(10, 8)
	c.Text(f, 0, 0, "é", Copy)
	d.Text(f, 0, 0, "?", Copy)
	for i := range c.ink {
		if c.ink[i] != d.ink[i] {
			t.Fatal("a rune the font lacks is not a question mark")
		}
	}
	// Text goes where it says, in the mode it says, and ends where it ended.
	cv := NewCanvas(40, 10)
	if end := cv.Text(f, 1, 1, "Hi 7", Copy); end != 1+f.Width("Hi 7") {
		t.Errorf("text ends at %d, want %d", end, 1+f.Width("Hi 7"))
	}
	// An unknown font is the usual one.
	if FontNamed("no such font").Name != "sans5x7" {
		t.Error("an unknown font is not the usual one")
	}
}

func TestFontFileMistakesAreSaid(t *testing.T) {
	for _, tc := range []struct{ src, want string }{
		{"font x\n\nA\n#", "no height"},
		{"font x\nheight 2\n\nA\n#\n#\n#", "3 rows"},
		{"font x\nheight 2\n\nA\n##\n#", "differ in width"},
		{"font x\nheight 2\nwiggle 3\n", "unknown setting"},
		{"font x\nheight two\n", "not a size"},
		{"font x\nheight 1\nkern A 1\n", "bad kern"},
		{"font x\nheight 1\n\nAB\n#", "one character"},
	} {
		_, err := ParseFont(tc.src)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%q: error %v, want one with %q", tc.src, err, tc.want)
		}
	}
}

func TestSheetsHaveSpritesAndAnimations(t *testing.T) {
	s, err := ParseSheet(`// a comment
sprite a
#.
.#

sprite b
##
..
anim blink 3 a b
`)
	if err != nil {
		t.Fatal(err)
	}
	if s.Image("a") == nil || s.Image("a").W != 2 || s.Image("nothing") != nil {
		t.Error("sprites are not as the sheet says")
	}
	an := s.Anim("blink")
	for tick, want := range []string{"a", "a", "a", "b", "b", "b", "a"} {
		if an.At(tick) != s.Image(want) {
			t.Errorf("tick %d shows the wrong frame, want %s", tick, want)
		}
	}
	if s.Anim("no such").At(5) != nil {
		t.Error("an animation of no frames shows something")
	}
	for _, bad := range []string{"anim x 3 nothing", "anim x 0 a", "anim x", "sprite "} {
		if _, err := ParseSheet("sprite a\n#\n\n" + bad); err == nil {
			t.Errorf("%q was taken", bad)
		}
	}
}

func TestTilemapDrawsItsCells(t *testing.T) {
	s := MustSheet("sprite x\n##\n##\n\nsprite o\n..\n..\n")
	tm := NewTilemap(3, 2, 2, 2)
	tm.Set(1, 0, "x")
	tm.Set(2, 1, "x")
	tm.Set(9, 9, "x") // off the map: nothing
	if tm.At(1, 0) != "x" || tm.At(0, 0) != "" || tm.At(9, 9) != "" {
		t.Error("cells are not as set")
	}
	cv := NewCanvas(8, 6)
	tm.Draw(cv, s, 1, 1, Copy)
	if !cv.Get(3, 1) || !cv.Get(6, 3) || cv.Get(1, 1) || cv.Get(0, 0) {
		t.Error("the tiles are not where the cells are")
	}
}

func TestThemes(t *testing.T) {
	if ThemeNamed("no such", false).Name != Themes[0].Name {
		t.Error("an unknown theme is not the first")
	}
	for _, th := range Themes {
		if th.Ink == th.Paper {
			t.Errorf("theme %s has no contrast", th.Name)
		}
		d := ThemeNamed(th.Name, true)
		if d.Ink == d.Paper {
			t.Errorf("theme %s has none in the dark", th.Name)
		}
	}
	light, dark := ThemeNamed("handheld", false), ThemeNamed("handheld", true)
	lum := func(r, g, b uint8) int { return int(r) + int(g) + int(b) }
	if lum(dark.Paper.R, dark.Paper.G, dark.Paper.B) >= lum(light.Paper.R, light.Paper.G, light.Paper.B) {
		t.Error("the paper is not darker in the dark")
	}
}
