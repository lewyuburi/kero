# Games

A game pane plays one of the games here (the `games` flag puts Games in the
command palette). The app knows none by name: the pane in `games.go` asks this
package for the catalog, and draws what a game says on a screen of pixels.

```
internal/pixel            the screen: canvas, images and modes, fonts, sheets, tilemaps, themes
internal/games            the interface a game keeps, and the catalog
internal/games/gallery    the game that shows the others: Games in the palette opens it
internal/games/snake      a game: engine, view, sprites
internal/games/blackice   BLACK ICE, defense by lanes, with its DESIGN.md
```

Games in the palette is the gallery, which has a tile for each game, with the
arrows or the mouse, and puts the one chosen in its pane. A game opens without
the gallery by name: `kero do pane.game game=snake`, or `kero split --url
kero-game://snake`; `pane.game` with no game is the gallery. From a game,
Escape, or the button in its header, goes back to the gallery.

## A new game

A package under `internal/games/<name>` with an engine, which knows nothing of
the window and is tested without one, and a view that draws it:

1. The engine is a `games.Game`: `Interval` (the time between its steps, 0 while
   it waits), `Step`, `Key` and `Status`. Steps are the game's clock, not frames:
   a game that is stepped at 30 a second does the same on any machine.
   Add `games.Animator` for something that moves between steps, as a shake.
2. The view is `Draw(g, canvas)`, and registers the game in `init` with
   `games.Register(games.Kind{...})`: its name, its title and its `Draw`. Leave
   the top `games.HUDHeight` pixels free: the pane draws the score and the hint
   there.
3. Add `_ "kero/internal/games/<name>"` to the imports in `games.go`. Give the
   `Kind` an `Icon`, 32 by 32 pixels, for its tile in the gallery.

The mouse: a game that implements `games.Pointed` is told of the pointer in
pixels of its screen, whatever the scale it is shown at: `PointerMove`,
`PointerDown`, `PointerUp` (after a Down, always) and `PointerLeave`. A game
that, rather than being played, leads to another implements `games.Chooser`.

Every screen is the same: 320 by 240 pixels, 4 to 3, which is 40 by 30 tiles of
8 pixels and 53 columns of text in the 6-pixel font. A game draws in the area
under the band at the top, 320 by 224. The pane shows the screen as large as
fits (settings.GameScale: at whole pixels of the display where that gives up
little, 2x is 640 by 480 and 3x 960 by 720), and never changes its proportions,
so that a game has nothing to say of its size. Snake's board is 19 by 13 cells
of 16 pixels in the middle of that area.

## What is data

- **Sprites**: a `.sheet` file, embedded with `go:embed` and read with
  `pixel.MustSheet`. `sprite name` and its rows (`#` ink, `.` paper, space
  transparent), `anim name ticks frame...` for an animation. Draw with
  `canvas.Draw(sheet.Image("name"), x, y, mode)`.
- **Fonts**: a `.font` file in `internal/pixel/fonts`, read by name with
  `pixel.FontNamed`. A header (`height`, `tracking`, `space`, `kern AV -1`) and
  a block to a glyph, as wide as its rows. A serif font is a new file; set
  `Font` in the game's `Kind` to use it.
- **Themes**: the colors of a screen, in `internal/pixel/theme.go`; set `Theme`
  in the `Kind`.

## Drawing

- Modes (`Copy`, `InkOnly`, `PaperOnly`, `FillInk`, `FillPaper`, `XOR`, `NXOR`,
  `Inverted`) apply to images and text, as a handheld's drawing modes do. Shapes
  (`Rect`, `Frame`, `Dither`) are drawn as they say.
- A shade is dithered (`Dither`), in a pattern that is the same wherever it is
  drawn. A shade from the half up is a checkerboard, which flashes if it moves a
  pixel at a time: move by pairs of pixels (`pixel.ShakeOffset` does), or draw
  the dither after moving.
