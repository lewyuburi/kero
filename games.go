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
	body := ui.Column(c.Key("game")).Grow(1).MinHeight(0).Center().Gap(6).Padding(0, 8, 10, 8).Focusable().Bind(ref)
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
	room := ref.Bounds(c)
	scale := float32(2)
	if room.W > 0 && room.H > 0 {
		scale = min((room.W-16)/float32(play.cv.w), (room.H-38)/float32(play.cv.h))
		// A whole multiple is crispest; the room it leaves is given up only
		// when it is a small part.
		if f := float32(int(scale)); f >= 1 && scale/f < 1.04 {
			scale = f
		}
		scale = max(scale, 1)
	}
	play.kind.draw(play.g, play.cv)
	ink, paper := screenColors(pal.dark)
	bm := play.cv.Bitmap(ink, paper)
	body.Children(func() {
		// The score and the hint share one row over the screen, as wide as
		// it is, so that the screen takes the rest of the room.
		w, h := scale*float32(play.cv.w), scale*float32(play.cv.h)
		ui.Row(c.Key("status")).Width(w).Height(20).AlignItems(ui.Center).Padding(0, 4).Children(func() {
			ui.Text(c, score).FontSize(15).FontWeight(700).TextColor(pal.text).SingleLine().FontFeatures("tnum")
			ui.Spacer(c)
			ui.Text(c, hint).FontSize(12.5).TextColor(pal.textMuted).SingleLine()
		})
		ui.Box(c.Key("screen")).Size(w, h).Draw(func(p *ui.Painter, r ui.Rect) {
			p.ImageRegion(bm, ui.Rect{}, r, ui.ImageOptions{})
		})
	})
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
