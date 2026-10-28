package pixel

import "image/color"

// A Theme is the two colors of a screen: its ink and its paper.
type Theme struct {
	Name       string
	Ink, Paper color.RGBA
}

// Themes are the screens' colors, by name. A dark one goes with each, which
// is the same with ink and paper the other way about, as it is in the dark.
var Themes = []Theme{
	{"handheld", color.RGBA{0x2f, 0x2d, 0x27, 0xff}, color.RGBA{0xb5, 0xb3, 0xa6, 0xff}},
	{"green", color.RGBA{0x0f, 0x38, 0x0f, 0xff}, color.RGBA{0x9b, 0xbc, 0x0f, 0xff}},
	{"amber", color.RGBA{0x2a, 0x14, 0x00, 0xff}, color.RGBA{0xff, 0xb0, 0x00, 0xff}},
	{"mono", color.RGBA{0x00, 0x00, 0x00, 0xff}, color.RGBA{0xff, 0xff, 0xff, 0xff}},
}

// ThemeNamed is the theme of that name, the first if there is none. In the
// dark, ink and paper trade places on the themes that are pale, and the
// paper of a dark one is kept dim, so the screen does not glare.
func ThemeNamed(name string, dark bool) Theme {
	t := Themes[0]
	for _, th := range Themes {
		if th.Name == name {
			t = th
		}
	}
	if dark {
		t.Ink, t.Paper = dim(t.Paper, 0.85), dim(t.Ink, 1)
		if t.Name == "handheld" {
			t.Paper = color.RGBA{0x24, 0x23, 0x1f, 0xff}
			t.Ink = color.RGBA{0xc9, 0xc7, 0xb9, 0xff}
		}
	}
	return t
}

// dim is a color made f of its brightness.
func dim(c color.RGBA, f float32) color.RGBA {
	return color.RGBA{uint8(float32(c.R) * f), uint8(float32(c.G) * f), uint8(float32(c.B) * f), c.A}
}
