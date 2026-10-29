package pixel

import (
	"embed"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"unicode"
)

// A Font is drawn in the pixels of the screen it is on, so it has the size of
// one of them and no other. Glyphs have a width of their own, and a few pairs
// are kerned.
type Font struct {
	Name     string
	Height   int // of every glyph
	Tracking int // pixels between glyphs
	Space    int // the width of a space
	glyphs   map[rune]*Image
	kern     map[[2]rune]int
}

// A font file starts with its header, one setting to a line, up to a blank
// line, and goes on with one glyph to a block: the character, then its rows,
// # for ink, anything else transparent. Glyphs are as wide as their rows.
//
//	font sans5x7
//	height 7
//	tracking 1
//	space 3
//	kern AV -1
//
//	A
//	.###.
//	#...#
//	...
//
// A line that starts with // is a comment.
func ParseFont(src string) (*Font, error) {
	f := &Font{glyphs: map[rune]*Image{}, kern: map[[2]rune]int{}, Tracking: 1, Space: 3}
	blocks := strings.Split(strings.ReplaceAll(strings.TrimSpace(src), "\r\n", "\n"), "\n\n")
	for _, line := range strings.Split(blocks[0], "\n") {
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		key, rest, _ := strings.Cut(line, " ")
		switch key {
		case "font":
			f.Name = rest
		case "height", "tracking", "space":
			n, err := strconv.Atoi(rest)
			if err != nil || n < 0 {
				return nil, fmt.Errorf("font %s: %q is not a size", f.Name, line)
			}
			switch key {
			case "height":
				f.Height = n
			case "tracking":
				f.Tracking = n
			case "space":
				f.Space = n
			}
		case "kern":
			var pair string
			var n int
			if _, err := fmt.Sscanf(rest, "%s %d", &pair, &n); err != nil || len([]rune(pair)) != 2 {
				return nil, fmt.Errorf("font %s: bad kern %q", f.Name, line)
			}
			r := []rune(pair)
			f.kern[[2]rune{r[0], r[1]}] = n
		default:
			return nil, fmt.Errorf("font %s: unknown setting %q", f.Name, line)
		}
	}
	if f.Height == 0 {
		return nil, fmt.Errorf("font %s has no height", f.Name)
	}
	for _, block := range blocks[1:] {
		var lines []string
		for _, l := range strings.Split(block, "\n") {
			if !strings.HasPrefix(l, "//") {
				lines = append(lines, l)
			}
		}
		if len(lines) == 0 {
			continue
		}
		head := []rune(lines[0])
		if len(head) != 1 {
			return nil, fmt.Errorf("font %s: %q does not name one character", f.Name, lines[0])
		}
		rows := lines[1:]
		if len(rows) != f.Height {
			return nil, fmt.Errorf("font %s: %q has %d rows, not %d", f.Name, string(head), len(rows), f.Height)
		}
		for _, r := range rows {
			if len([]rune(r)) != len([]rune(rows[0])) {
				return nil, fmt.Errorf("font %s: rows of %q differ in width", f.Name, string(head))
			}
		}
		f.glyphs[head[0]] = ParseImage(rows, false)
	}
	return f, nil
}

//go:embed fonts/*.font
var fontFiles embed.FS

var (
	fontMu sync.Mutex
	fonts  = map[string]*Font{}
)

// FontNamed is the font of that name, from fonts/: sans5x7 and tiny3x5 are
// there. An unknown name is the first of those, which is what the screens use.
func FontNamed(name string) *Font {
	b, err := fontFiles.ReadFile("fonts/" + name + ".font")
	if err != nil {
		name = "sans5x7"
		b, _ = fontFiles.ReadFile("fonts/sans5x7.font")
	}
	fontMu.Lock()
	defer fontMu.Unlock()
	if f := fonts[name]; f != nil {
		return f
	}
	f, err := ParseFont(string(b))
	if err != nil {
		panic("pixel: " + err.Error())
	}
	fonts[name] = f
	return f
}

// glyph is the glyph of a rune: its capital for a letter the font has none
// of, a question mark for what it lacks, nothing for a space.
func (f *Font) glyph(r rune) *Image {
	if g, ok := f.glyphs[r]; ok {
		return g
	}
	if g, ok := f.glyphs[unicode.ToUpper(r)]; ok {
		return g
	}
	return f.glyphs['?']
}

// Width is the width of s in pixels: its glyphs and what is between them,
// with none after the last.
func (f *Font) Width(s string) int {
	w, prev := 0, rune(0)
	for i, r := range []rune(s) {
		if i > 0 {
			w += f.Tracking + f.kern[[2]rune{unicode.ToUpper(prev), unicode.ToUpper(r)}]
		}
		w += f.advance(r)
		prev = r
	}
	return w
}

func (f *Font) advance(r rune) int {
	if r == ' ' {
		return f.Space
	}
	if g := f.glyph(r); g != nil {
		return g.W
	}
	return f.Space
}

// Text draws s with its top left at x, y in the mode given, and returns
// where it ends.
func (cv *Canvas) Text(f *Font, x, y int, s string, m Mode) int {
	prev := rune(0)
	for i, r := range []rune(s) {
		if i > 0 {
			x += f.Tracking + f.kern[[2]rune{unicode.ToUpper(prev), unicode.ToUpper(r)}]
		}
		if r != ' ' {
			cv.Draw(f.glyph(r), x, y, m)
		}
		x += f.advance(r)
		prev = r
	}
	return x
}

// Tooltip draws a box of text, lines of it, for a tooltip drawn on the screen as
// the screen is: a frame in ink, paper inside it, and the text in the font.
// anchorX, anchorY is what it is about; the box is centered over it, above if
// there is room (not above top), else below, and always inside the canvas. It
// returns where the box is.
func (cv *Canvas) Tooltip(f *Font, anchorX, anchorY, top int, lines ...string) (x, y, w, h int) {
	const pad = 3
	tw := 0
	for _, l := range lines {
		tw = max(tw, f.Width(l))
	}
	lh := f.Height + 2
	w, h = tw+2*pad+2, len(lines)*lh-2+2*pad+2
	x = min(max(anchorX-w/2, 1), cv.W-w-1)
	y = anchorY - h - 4
	if y < top {
		y = anchorY + 14
	}
	y = min(max(y, 1), cv.H-h-1)
	cv.Frame(x, y, w, h, 1)
	for i, l := range lines {
		cv.Text(f, x+1+pad, y+1+pad+i*lh, l, Copy)
	}
	return x, y, w, h
}
