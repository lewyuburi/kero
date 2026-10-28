// Package games is what a game pane plays. A game is an engine that knows
// nothing of the window, and a view that draws its screen in pixels
// (internal/pixel); it registers itself, with a name, and the pane needs to
// know no more than this package says. A game of its own is a package under
// here that calls Register, and a blank import in the app.
package games

import (
	"time"

	"kero/internal/pixel"
)

// Key is what a player can press, whatever the game.
type Key int

const (
	Up Key = iota
	Down
	Left
	Right
	Action  // space: pause, or begin
	Restart // enter
)

// A Game steps at the interval it asks, takes keys, and tells the player how
// it stands.
type Game interface {
	// Interval is the time between steps, 0 while it waits for a key. The
	// steps are the game's own clock: they are not frames, and what is
	// drawn does not set their pace.
	Interval() time.Duration
	Step()
	Key(k Key)
	// Status is the score and a hint, which is empty while the game is on.
	Status() (score, hint string)
}

// An Animator is a game with something moving on its screen between steps,
// as a shake: while Animating, the pane asks for a frame in each Frame's
// time, and Frame lets one go by.
type Animator interface {
	Animating() bool
	Frame()
}

// FrameTime is the time of a frame of what Animators show: 30 a second.
const FrameTime = time.Second / 30

// Every screen is the same size, 256 by 192 pixels, 4 to 3: the screen of
// the handhelds that had one, and a shape that fits a pane of any. It is 32
// by 24 tiles of 8 pixels, or 16 by 12 of 16, and a font of 6 pixels to a
// character has 42 columns of text on it. A game draws in the area under the
// band at the top, where the pane draws the score and the hint: Area.
const (
	Width, Height = 256, 192
	// HUDHeight is that band.
	HUDHeight = 16
	// AreaHeight is what is left under it, 256 by 176: 32 by 22 tiles of 8.
	AreaHeight = Height - HUDHeight
)

// A Kind is a game that can be played.
type Kind struct {
	ID, Title string
	New       func() Game
	// Draw paints the game on a screen of Width by Height.
	Draw func(g Game, cv *pixel.Canvas)
	// Font is the font of the score and the hint, "" for the usual.
	Font string
	// Theme is the colors of the screen, "" for the usual.
	Theme string
}

var kinds []Kind

// Register adds a game to the catalog.
func Register(k Kind) { kinds = append(kinds, k) }

// Kinds are the games there are, in the order they were registered.
func Kinds() []Kind { return append([]Kind(nil), kinds...) }

// Lookup is the game with that ID.
func Lookup(id string) (Kind, bool) {
	for _, k := range kinds {
		if k.ID == id {
			return k, true
		}
	}
	return Kind{}, false
}
