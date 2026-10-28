package main

import (
	"strings"
	"unicode"
)

// The font of a game's screen is drawn in its pixels, as the rest of it is,
// so it has the size of a pixel of that screen, and no other: glyphs 5
// pixels wide and 7 tall, a pixel apart. It has capitals, digits and a few
// marks, which is what a score and a hint say.
const (
	glyphW, glyphH = 5, 7
	glyphAdvance   = glyphW + 1
)

// pixelGlyphs are the glyphs by rune.
var pixelGlyphs = func() map[rune][glyphH]string {
	m := map[rune][glyphH]string{}
	for _, block := range strings.Split(strings.TrimSpace(glyphSource), "\n\n") {
		lines := strings.Split(block, "\n")
		var g [glyphH]string
		copy(g[:], lines[1:])
		m[[]rune(lines[0])[0]] = g
	}
	return m
}()

// glyphSource is one glyph to a block: the character, then its seven rows,
// # for ink.
const glyphSource = `
A
.###.
#...#
#...#
#####
#...#
#...#
#...#

B
####.
#...#
#...#
####.
#...#
#...#
####.

C
.###.
#...#
#....
#....
#....
#...#
.###.

D
####.
#...#
#...#
#...#
#...#
#...#
####.

E
#####
#....
#....
####.
#....
#....
#####

F
#####
#....
#....
####.
#....
#....
#....

G
.###.
#...#
#....
#.###
#...#
#...#
.###.

H
#...#
#...#
#...#
#####
#...#
#...#
#...#

I
.###.
..#..
..#..
..#..
..#..
..#..
.###.

J
..###
...#.
...#.
...#.
...#.
#..#.
.##..

K
#...#
#..#.
#.#..
##...
#.#..
#..#.
#...#

L
#....
#....
#....
#....
#....
#....
#####

M
#...#
##.##
#.#.#
#.#.#
#...#
#...#
#...#

N
#...#
##..#
#.#.#
#..##
#...#
#...#
#...#

O
.###.
#...#
#...#
#...#
#...#
#...#
.###.

P
####.
#...#
#...#
####.
#....
#....
#....

Q
.###.
#...#
#...#
#...#
#.#.#
#..#.
.##.#

R
####.
#...#
#...#
####.
#.#..
#..#.
#...#

S
.####
#....
#....
.###.
....#
....#
####.

T
#####
..#..
..#..
..#..
..#..
..#..
..#..

U
#...#
#...#
#...#
#...#
#...#
#...#
.###.

V
#...#
#...#
#...#
#...#
#...#
.#.#.
..#..

W
#...#
#...#
#...#
#.#.#
#.#.#
##.##
#...#

X
#...#
#...#
.#.#.
..#..
.#.#.
#...#
#...#

Y
#...#
#...#
.#.#.
..#..
..#..
..#..
..#..

Z
#####
....#
...#.
..#..
.#...
#....
#####

0
.###.
#...#
#..##
#.#.#
##..#
#...#
.###.

1
..#..
.##..
..#..
..#..
..#..
..#..
.###.

2
.###.
#...#
....#
...#.
..#..
.#...
#####

3
####.
....#
....#
.###.
....#
....#
####.

4
...#.
..##.
.#.#.
#..#.
#####
...#.
...#.

5
#####
#....
####.
....#
....#
#...#
.###.

6
.###.
#....
#....
####.
#...#
#...#
.###.

7
#####
....#
...#.
..#..
.#...
.#...
.#...

8
.###.
#...#
#...#
.###.
#...#
#...#
.###.

9
.###.
#...#
#...#
.####
....#
....#
.###.

.
.....
.....
.....
.....
.....
.##..
.##..

,
.....
.....
.....
.....
.##..
..#..
.#...

-
.....
.....
.....
#####
.....
.....
.....

:
.....
.##..
.##..
.....
.##..
.##..
.....

!
..#..
..#..
..#..
..#..
..#..
.....
..#..

?
.###.
#...#
....#
...#.
..#..
.....
..#..

/
....#
....#
...#.
..#..
.#...
#....
#....

+
.....
..#..
..#..
#####
..#..
..#..
.....

·
.....
.....
.....
..#..
.....
.....
.....
`

// glyphFor is the glyph of a rune: its capital for a letter, nothing for a
// space, a question mark for what the font has not.
func glyphFor(r rune) ([glyphH]string, bool) {
	if r == ' ' {
		return [glyphH]string{}, false
	}
	if g, ok := pixelGlyphs[unicode.ToUpper(r)]; ok {
		return g, true
	}
	return pixelGlyphs['?'], true
}

// TextWidth is the width of s in pixels, with no space after its last glyph.
func (cv *pixelCanvas) TextWidth(s string) int {
	n := len([]rune(s))
	if n == 0 {
		return 0
	}
	return n*glyphAdvance - 1
}

// Text draws s in ink with its top left at x, y, and returns where it ends.
func (cv *pixelCanvas) Text(x, y int, s string) int {
	for _, r := range s {
		if g, ok := glyphFor(r); ok {
			for j, row := range g {
				for i, c := range row {
					if c == '#' {
						cv.Set(x+i, y+j, true)
					}
				}
			}
		}
		x += glyphAdvance
	}
	return x - 1
}
