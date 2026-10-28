package main

import (
	"time"

	"github.com/egoist/mygo/ui"

	"kero/internal/proto"
	"kero/internal/settings"
	"kero/locales"
)

// A game pane is a browser pane's kin: the daemon keeps it as a pane with a
// URL and no process (proto.Pane.Game), so that it splits, moves, closes and
// comes back as any pane does, and the app draws the game here instead of a
// page. What a game is lives in its own file; this one is the pane.

// hudHeight is the band at the top of a game's screen where the score and
// the hint are drawn, which a game leaves free.
const hudHeight = 12

// gameKind is a game a pane can play.
type gameKind struct {
	id, title string
	new       func() game
	// canvas is the size of the game's screen in pixels, and draw paints it.
	// The screen keeps its proportions in any pane.
	canvas func() (w, h int)
	draw   func(g game, cv *pixelCanvas)
}

var gameKinds = []gameKind{
	{"snake", "Snake", func() game { return newSnake(time.Now().UnixNano()) }, snakeCanvas, drawSnake},
}

// gamePlay is a game being played in a pane, and where its clock stands.
type gamePlay struct {
	g       game
	kind    gameKind
	cv      *pixelCanvas
	last    time.Time // when it last stepped
	running bool      // it was stepping in the last frame
	focused bool
}

// gameFor is the game a pane plays, made as it is first shown; nil for a
// name no game has.
func (a *App) gameFor(p *proto.Pane) *gamePlay {
	if play := a.games[p.ID]; play != nil {
		return play
	}
	for _, k := range gameKinds {
		if k.id == p.Game() {
			if a.games == nil {
				a.games = map[string]*gamePlay{}
			}
			w, h := k.canvas()
			play := &gamePlay{g: k.new(), kind: k, cv: newPixelCanvas(w, h)}
			a.games[p.ID] = play
			return play
		}
	}
	return nil
}

// openGame splits the pane in focus with a pane playing a game, or with no
// session, starts one that is only the game, in the home folder as a new
// session with no project to go by starts.
func (a *App) openGame(id string) {
	s := a.session(a.selected)
	if s == nil {
		h := a.selectedHost()
		home := proto.Home()
		if h.remote() {
			home = "~"
		}
		a.create(h, proto.CreateParams{Dir: home, Loose: looseOn(a.set), URL: proto.GameURL(id)}, "")
		return
	}
	a.split(s, proto.CreateParams{Split: a.focusOf(s), Direction: "right", URL: proto.GameURL(id)})
}

// gameItems are the palette's entries for starting a game, with the flag.
func (a *App) gameItems() []paletteItem {
	if !a.set.Flag(settings.FlagGames) {
		return nil
	}
	var items []paletteItem
	for _, k := range gameKinds {
		id := k.id
		items = append(items, paletteItem{title: locales.NewGame(k.title), words: "game play " + id, run: func() { a.openGame(id) }})
	}
	return items
}

// gameKeys are the keys a game takes.
var gameKeys = []struct {
	key  ui.Key
	game gameKey
}{
	{ui.KeyUp, keyUp}, {ui.KeyW, keyUp},
	{ui.KeyDown, keyDown}, {ui.KeyS, keyDown},
	{ui.KeyLeft, keyLeft}, {ui.KeyA, keyLeft},
	{ui.KeyRight, keyRight}, {ui.KeyD, keyRight},
	{ui.KeySpace, keyAction}, {ui.KeyEnter, keyRestart},
}

// paneGame is a game pane's body, under the header a pane has: the board
// in a monospaced font, and a line of status. It takes the keyboard as a
// terminal does, steps while it has it, and waits where it is when the
// player is in another pane.
func (a *App) paneGame(c *ui.Context, pal *palette, s *proto.Session, p *proto.Pane) {
	play := a.gameFor(p)
	ref := a.paneRef(p.ID)
	body := ui.Column(c.Key("game")).Grow(1).MinHeight(0).Center().Padding(0, 8, 10, 8).Focusable().Bind(ref)
	if play == nil {
		body.Children(func() {
			ui.Text(c, p.Game()).FontSize(13).TextColor(pal.textFaint)
		})
		return
	}
	focused := ref.Focused(c)
	if focused && a.focusOf(s) != p.ID {
		a.setFocus(s, p.ID)
	}
	for _, k := range gameKeys {
		if body.Shortcut(0, k.key) {
			play.g.Key(k.game)
		}
	}
	a.stepGame(c, play, focused)
	score, hint := play.g.Status()
	play.kind.draw(play.g, play.cv)
	// The score at the left and the hint at the right, in the screen's font.
	play.cv.Text(4, 3, score)
	play.cv.Text(play.cv.w-4-play.cv.TextWidth(hint), 3, hint)
	ink, paper := screenColors(pal.dark)
	bm := play.cv.Bitmap(ink, paper)
	body.Children(func() {
		ui.Box(c.Key("screen")).Grow(1).MinWidth(0).MinHeight(0).AlignSelf(ui.Stretch).Draw(func(p *ui.Painter, r ui.Rect) {
			drawScreen(p, r, bm, play.cv.w, play.cv.h)
		})
	})
}

// drawScreen shows a screen of w by h pixels in the middle of r, as large as
// fits at a whole number of the display's pixels to a pixel of the screen:
// each is then a square of whole pixels of the display, none wider than the
// next, as on a handheld, and the screen's edges fall between pixels. Where
// the display has more than one pixel to a DIP that lets the scale go in
// steps smaller than a DIP, so that less of the room is given up.
func drawScreen(p *ui.Painter, r ui.Rect, bm *ui.Bitmap, w, h int) {
	dpr := max(p.Scale(), 1)
	fit := min(r.W/float32(w), r.H/float32(h))
	scale := max(float32(int(fit*dpr))/dpr, 1/dpr)
	sw, sh := scale*float32(w), scale*float32(h)
	dst := ui.Rect{
		X: r.X + float32(int((r.W-sw)*dpr/2))/dpr,
		Y: r.Y + float32(int((r.H-sh)*dpr/2))/dpr,
		W: sw, H: sh,
	}
	p.ImageRegion(bm, ui.Rect{}, dst, ui.ImageOptions{})
}

// stepGame advances a game by the time that has passed while it had the
// keyboard, and asks for the frame after its next step. A game that loses
// the keyboard stops, and goes on from where it was when it has it again.
func (a *App) stepGame(c *ui.Context, play *gamePlay, focused bool) {
	now := c.Now()
	iv := play.g.Interval()
	if !focused || iv == 0 || !play.running || focused != play.focused {
		play.last = now
	}
	play.focused = focused
	play.running = focused && iv > 0
	if !play.running {
		return
	}
	for n := 0; n < 4 && now.Sub(play.last) >= iv; n++ {
		play.g.Step()
		play.last = play.last.Add(iv)
		if iv = play.g.Interval(); iv == 0 {
			play.running = false
			return
		}
	}
	if now.Sub(play.last) >= iv {
		play.last = now // far behind: do not catch up
	}
	c.After(max(iv-now.Sub(play.last), time.Millisecond))
}
