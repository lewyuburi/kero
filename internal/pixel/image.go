package pixel

import "strings"

// An Image is a picture of ink and paper with a mask: a pixel is opaque, as
// ink or as paper, or transparent, which one bit has no way to say of itself.
type Image struct {
	W, H int
	ink  []bool
	mask []bool // opaque
}

// ParseImage reads rows of characters: # is ink, and . is paper where
// paperOpaque, else transparent like any other character.
func ParseImage(rows []string, paperOpaque bool) *Image {
	img := &Image{H: len(rows)}
	for _, r := range rows {
		img.W = max(img.W, len([]rune(r)))
	}
	img.ink, img.mask = make([]bool, img.W*img.H), make([]bool, img.W*img.H)
	for y, r := range rows {
		for x, c := range []rune(r) {
			switch {
			case c == '#':
				img.ink[y*img.W+x], img.mask[y*img.W+x] = true, true
			case c == '.' && paperOpaque:
				img.mask[y*img.W+x] = true
			}
		}
	}
	return img
}

// A Mode says how an image or text is drawn over what is there, as the
// drawing modes of a handheld do: it applies to images and text and not to
// shapes.
type Mode int

const (
	Copy      Mode = iota // as it is
	InkOnly               // only its ink: its paper is transparent
	PaperOnly             // only its paper: its ink is transparent
	FillInk               // everything opaque in ink, the image as a stencil
	FillPaper             // everything opaque in paper
	XOR                   // its ink turns what is under it over
	NXOR                  // its paper does
	Inverted              // as it is, ink for paper and paper for ink
)

// Draw draws an image with its top left at x, y.
func (cv *Canvas) Draw(img *Image, x, y int, m Mode) {
	if img == nil {
		return
	}
	for j := 0; j < img.H; j++ {
		for i := 0; i < img.W; i++ {
			k := j*img.W + i
			if !img.mask[k] {
				continue
			}
			src, px, py := img.ink[k], x+i, y+j
			switch m {
			case Copy:
				cv.Set(px, py, src)
			case InkOnly:
				if src {
					cv.Set(px, py, true)
				}
			case PaperOnly:
				if !src {
					cv.Set(px, py, false)
				}
			case FillInk:
				cv.Set(px, py, true)
			case FillPaper:
				cv.Set(px, py, false)
			case XOR:
				if src {
					cv.Set(px, py, !cv.Get(px, py))
				}
			case NXOR:
				if !src {
					cv.Set(px, py, !cv.Get(px, py))
				}
			case Inverted:
				cv.Set(px, py, !src)
			}
		}
	}
}

// Rows is the image as the characters it is read from: # ink, . paper, a
// space transparent.
func (img *Image) Rows() []string {
	rows := make([]string, img.H)
	for y := range rows {
		var b strings.Builder
		for x := 0; x < img.W; x++ {
			switch k := y*img.W + x; {
			case !img.mask[k]:
				b.WriteByte(' ')
			case img.ink[k]:
				b.WriteByte('#')
			default:
				b.WriteByte('.')
			}
		}
		rows[y] = b.String()
	}
	return rows
}
