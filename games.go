package main

import (
	"image"
	"time"

	"github.com/egoist/mygo/ui"

	"kero/internal/games"
	_ "kero/internal/games/snake" // the games there are: each registers itself
	"kero/internal/pixel"
	"kero/internal/proto"
	"kero/internal/settings"
	"kero/locales"
)

// A game pane is a browser pane's kin: the daemon keeps it as a pane with a
// URL and no process (proto.Pane.Game), so that it splits, moves, closes and
// comes back as any pane does, and the app draws the game here instead of a
// page. What a game is lives in internal/games; this file is the pane, and
// knows of no game by name.

// gamePlay is a game being played in a pane, and where its clock stands.
type gamePlay struct {
	g       games.Game
	kind    games.Kind
	cv      *pixel.Canvas
	img     *image.RGBA
	bm      *ui.Bitmap
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
	k, ok := games.Lookup(p.Game())
	if !ok {
		return nil
	}
	if a.games == nil {
		a.games = map[string]*gamePlay{}
	}
	play := &gamePlay{g: k.New(), kind: k, cv: pixel.NewCanvas(games.Width, games.Height)}
	a.games[p.ID] = play
	return play
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
	for _, k := range games.Kinds() {
		id := k.ID
		items = append(items, paletteItem{title: locales.NewGame(k.Title), words: "game play " + id, run: func() { a.openGame(id) }})
	}
	return items
}

// gameKeys are the keys a game takes.
var gameKeys = []struct {
	key  ui.Key
	game games.Key
}{
	{ui.KeyUp, games.Up}, {ui.KeyW, games.Up},
	{ui.KeyDown, games.Down}, {ui.KeyS, games.Down},
	{ui.KeyLeft, games.Left}, {ui.KeyA, games.Left},
	{ui.KeyRight, games.Right}, {ui.KeyD, games.Right},
	{ui.KeySpace, games.Action}, {ui.KeyEnter, games.Restart},
}

// paneGame is a game pane's body, under the header a pane has: the screen of
// the game, in the paper of its colors. It takes the keyboard as a terminal
// does, steps while it has it, and waits where it is when the player is in
// another pane.
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
	play.kind.Draw(play.g, play.cv)
	// The score at the left and the hint at the right, in the screen's font,
	// centered in the band the game leaves for them.
	font := pixel.FontNamed(play.kind.Font)
	y := (games.HUDHeight - font.Height + 1) / 2
	play.cv.Text(font, 4, y, score, pixel.Copy)
	play.cv.Text(font, play.cv.W-4-font.Width(hint), y, hint, pixel.Copy)
	theme := pixel.ThemeNamed(play.kind.Theme, pal.dark)
	play.img = play.cv.RGBA(theme.Ink, theme.Paper, play.img)
	if play.bm == nil {
		play.bm = ui.NewBitmap(play.img).Pixelated()
	} else {
		play.bm.Update(play.img)
	}
	body.Children(func() {
		ui.Box(c.Key("screen")).Grow(1).MinWidth(0).MinHeight(0).AlignSelf(ui.Stretch).Draw(func(p *ui.Painter, r ui.Rect) {
			p.ImageRegion(play.bm, ui.Rect{}, screenRect(r, p.Scale(), play.cv.W, play.cv.H, a.set.GameScale), ui.ImageOptions{})
		})
	})
}

// gamePaper is the paper of a game pane's card: the screen's, so that the
// screen goes to the card's edges.
func (a *App) gamePaper(p *proto.Pane, dark bool) ui.Color {
	theme := pixel.ThemeNamed("", dark)
	if k, ok := games.Lookup(p.Game()); ok {
		theme = pixel.ThemeNamed(k.Theme, dark)
	}
	return ui.RGB(theme.Paper.R, theme.Paper.G, theme.Paper.B)
}

// snapKeeps is how much of the room a whole scale must keep, of what filling
// would take, for snap to choose it: one in ten is given up for crisp pixels.
const snapKeeps = 0.9

// screenRect is where a screen of w by h pixels goes in the middle of r, in
// proportion, as large as fits (settings.GameScale). Filling, the scale is
// whatever fits; whole, it is a whole number of the display's pixels to a
// pixel of the screen, so that each is a square of whole pixels of the
// display, none wider than the next, as on a handheld; snap fills, but is
// whole when whole keeps most of what filling takes. dpr is how many pixels of
// the display a DIP is; where it is more than one the whole scales go in steps
// smaller than a DIP. The size and the place are in whole pixels of the
// display, so that the edges fall between them.
func screenRect(r ui.Rect, dpr float32, w, h int, mode string) ui.Rect {
	dpr = max(dpr, 1)
	fit := min(r.W/float32(w), r.H/float32(h))
	whole := float32(int(fit*dpr)) / dpr
	scale := fit
	switch {
	case mode == settings.GameScaleWhole, mode == settings.GameScaleSnap && whole >= fit*snapKeeps:
		scale = whole
	}
	scale = max(scale, 1/dpr)
	sw, sh := float32(int(scale*float32(w)*dpr))/dpr, float32(int(scale*float32(h)*dpr))/dpr
	return ui.Rect{
		X: r.X + float32(int((r.W-sw)*dpr/2))/dpr,
		Y: r.Y + float32(int((r.H-sh)*dpr/2))/dpr,
		W: sw, H: sh,
	}
}

// stepGame advances a game by the time that has passed while it had the
// keyboard, and asks for the frame after its next step. A game that loses
// the keyboard stops, and goes on from where it was when it has it again. A
// game with something moving on its screen between steps (games.Animator)
// has a frame of it in each frame time, with the keyboard or not.
func (a *App) stepGame(c *ui.Context, play *gamePlay, focused bool) {
	if an, ok := play.g.(games.Animator); ok {
		if an.Animating() {
			an.Frame()
		}
		// What a step starts moving is drawn in the frames after it.
		defer func() {
			if an.Animating() {
				c.After(games.FrameTime)
			}
		}()
	}
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
