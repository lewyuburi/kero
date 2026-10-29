// Package kernelpanic is KERNEL PANIC, a game of defense by lanes: malware comes
// down five conduits of a fallen mainframe, and programs the player places
// hold it off. The engine here knows nothing of the window; see DESIGN.md for
// the game, and view.go for how it is drawn.
package kernelpanic

import (
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"time"

	"kero/internal/games"
)

// The sector, in tiles, and where everything is on the screen (320 by 240).
const (
	Cols, Rows   = 9, 5
	TileW, TileH = 28, 28
	// A sprite is 24 by 24 and a spark 14 by 14, a chip with a border that stands
	// out of whatever is under it, in the middle of a tile.
	spriteSize = 24
	spriteOff  = (TileW - spriteSize) / 2
	sparkSize  = 14
	sparkOff   = (TileW - sparkSize) / 2
	// The sector's top left, under the band of the score and the hint; the
	// cards' row under it, near the pointer that plays in the sector; and
	// under them the core and the waves.
	sectorX, sectorY = (games.Width - Cols*TileW) / 2, games.HUDHeight + 5
	cardsY           = sectorY + Rows*TileH + 8
	cardW, cardH     = 48, 32
	cardGap          = 3
	infoY            = cardsY + cardH + 4
)

// A step is a 30th of a second; the engine counts everything in steps.
const (
	StepTime = time.Second / 30
	perSec   = 30
)

// Kind is a program: what a card places.
type Kind int

const (
	Pulse Kind = iota
	Dynamo
	Wall
	Mine
	Cryo
	kinds
)

var programs = [kinds]struct {
	name  string
	cost  int
	ready int     // steps before the card can be used again
	hp    float64 // what it takes to eat it
	every int     // steps between its bolts or sparks
	title string
}{
	Pulse:  {"PULSE", 50, 5 * perSec, 12, 42, "pulse"},
	Dynamo: {"DYNAMO", 25, 4 * perSec, 12, 9 * perSec, "dynamo"},
	Wall:   {"WALL", 50, 20 * perSec, 80, 0, "wall"},
	Mine:   {"MINE", 25, 30 * perSec, 1, 0, "mine"},
	Cryo:   {"CRYO", 75, 5 * perSec, 12, 42, "cryo"},
}

// What the watts are written with: a bolt, which the fonts have a glyph for; and
// what a figure's width of nothing is, for them to blink.
const (
	wattSign    = '⚡'
	blankFigure = '\u2007'
)

// Name is the card's name.
func (k Kind) Name() string { return programs[k].name }

// Cost is the watts a program costs.
func (k Kind) Cost() int { return programs[k].cost }

// Malware.
type enemyKind int

const (
	Drone enemyKind = iota
	Worm
	Trojan
	enemyKinds
)

var malware = [enemyKinds]struct {
	hp    float64
	pace  float64 // tiles a step
	eats  float64 // health a step of the program it is at
	title string
}{
	Drone:  {20, 1.0 / 140, 4.0 / perSec, "drone"},
	Worm:   {12, 1.0 / 80, 3.0 / perSec, "worm"},
	Trojan: {56, 1.0 / 160, 5.0 / perSec, "trojan"},
}

const (
	boltPace      = 0.25 // tiles a step
	boltDamage    = 2.0
	cryoDamage    = 1.0
	slowSteps     = 3 * perSec
	mineArms      = 4 * perSec
	mineBlast     = 150.0
	sparkValue    = 25
	skySparkEvery = 9 * perSec
	sparkLife     = 14 * perSec
	startWatts    = 100
	startCore     = 3
	firstWave     = 20 * perSec
	waveEvery     = 25 * perSec
	waves         = 8
	shakeFrames   = 12
	dragSlop      = 3  // pixels of the screen
	tipDelay      = 12 // frames on a card before its tooltip shows
	blinkFrames   = 18 // the watts blink three times as they change
	flashFrames   = 6
)

type defender struct {
	kind  Kind
	hp    float64
	cd    int // steps to its next bolt or spark
	armed int // steps left before a mine is armed
	// holding is a dynamo that has made a spark and waits, with nothing more to
	// make, until it is taken or lost.
	holding bool
}

type enemy struct {
	kind enemyKind
	lane int
	x    float64 // its front, in tiles from the left of the sector
	hp   float64
	slow int
}

type bolt struct {
	lane int
	x    float64
	cryo bool
}

type spark struct {
	x, y  float64 // pixels of the screen
	ttl   int
	value int
	owner int // 1 and the tile of the dynamo that made it (r*Cols+c), 0 from the sky
}

type spawn struct {
	at   int
	kind enemyKind
	lane int
}

type card struct {
	kind  Kind
	ready int // the step it can be used from
}

// A game is one sector, held or lost.
type game struct {
	rng      *rand.Rand
	tick     int
	started  bool
	paused   bool
	won      bool
	over     bool
	watts    int
	core     int
	cells    [Rows][Cols]*defender
	enemies  []*enemy
	bolts    []bolt
	sparks   []spark
	cards    [kinds]card
	spawns   []spawn
	next     int // the next spawn to come
	nextSky  int
	selected int // the card in hand, -1 for none
	curC     int // the keyboard's tile
	curR     int
	// What the pointer is over and doing.
	hoverC, hoverR int // the tile, -1 for none
	hoverCard      int
	downCard       int // the card the pointer went down on, -1 for none
	downSpark      bool
	// A card dragged: where the pointer went down, and where it is. A press
	// that moves more than dragSlop pixels is a drag, and not a click.
	dragging     bool
	downX, downY int
	dragX, dragY int
	shake, flash int
	wattsBlink   int
	hoverTicks   int // frames the pointer has been on the card it is on
	flashLane    int
	best         int
	message      string // what the last try at placing said, for the hint
}

// New is a game with the waves it deals from seed.
func New(seed int64) games.Game { return newGame(seed) }

func newGame(seed int64) *game {
	g := &game{rng: rand.New(rand.NewSource(seed))}
	g.reset()
	return g
}

func (g *game) reset() {
	g.tick, g.started, g.paused, g.won, g.over = 0, false, false, false, false
	g.watts, g.core = startWatts, startCore
	g.cells = [Rows][Cols]*defender{}
	g.enemies, g.bolts, g.sparks = nil, nil, nil
	for k := Kind(0); k < kinds; k++ {
		g.cards[k] = card{kind: k}
	}
	g.selected, g.curC, g.curR = -1, 0, Rows/2
	g.hoverC, g.hoverR, g.hoverCard, g.downCard = -1, -1, -1, -1
	g.downSpark, g.dragging = false, false
	g.shake, g.flash, g.wattsBlink, g.message = 0, 0, 0, ""
	g.nextSky = skySparkEvery / 2
	g.deal()
}

// deal makes the waves: drones alone, then worms, then trojans, in groups that
// grow, the last of all three. Where in the wave each comes, and in which
// conduit, is the rng's.
func (g *game) deal() {
	g.spawns, g.next = nil, 0
	for w := 0; w < waves; w++ {
		var group []enemyKind
		add := func(k enemyKind, n int) {
			for i := 0; i < n; i++ {
				group = append(group, k)
			}
		}
		switch w {
		case 0:
			add(Drone, 2)
		case 1:
			add(Drone, 3)
		case 2:
			add(Drone, 3)
			add(Worm, 1)
		case 3:
			add(Drone, 4)
			add(Worm, 2)
		case 4:
			add(Trojan, 2)
			add(Drone, 3)
		case 5:
			add(Trojan, 3)
			add(Worm, 3)
		case 6:
			add(Trojan, 4)
			add(Drone, 5)
		default:
			add(Trojan, 6)
			add(Drone, 6)
			add(Worm, 4)
		}
		start := firstWave + w*waveEvery
		for _, k := range group {
			g.spawns = append(g.spawns, spawn{at: start + g.rng.Intn(10*perSec), kind: k, lane: g.rng.Intn(Rows)})
		}
	}
	sort.SliceStable(g.spawns, func(i, j int) bool { return g.spawns[i].at < g.spawns[j].at })
}

// Interval is a step while the sector is being held, and nothing while it
// waits for the player, or is won or lost.
func (g *game) Interval() time.Duration {
	if !g.started || g.over || g.won || g.paused {
		return 0
	}
	return StepTime
}

func (g *game) Animating() bool {
	return g.shake > 0 || g.flash > 0 || g.wattsBlink > 0 || (g.hoverCard >= 0 && g.hoverTicks < tipDelay)
}

// dynamoSprite is the sprite of a dynamo at a step: working, its bolt goes
// through three frames, dim, half, and nearly solid, in a second, and again, to
// say to the player that it is making a spark; once it has, it is still, with a
// solid bolt, until the spark is taken. The frames are the game's steps, so that
// they stop when the game does.
func dynamoSprite(d *defender, tick int) string {
	if d.holding {
		return "dynamo"
	}
	return [...]string{"dynamo-1", "dynamo-2", "dynamo-3"}[(tick/10)%3]
}

// tipShown tells whether the tooltip of the card under the pointer shows: after
// the pointer has stayed on it a moment, and not while a card is dragged.
func (g *game) tipShown() bool {
	return g.hoverCard >= 0 && g.hoverTicks >= tipDelay && !g.dragging
}

func (g *game) Frame() {
	g.shake, g.flash, g.wattsBlink = max(g.shake-1, 0), max(g.flash-1, 0), max(g.wattsBlink-1, 0)
	if g.hoverCard >= 0 && g.hoverTicks < tipDelay {
		g.hoverTicks++
	}
}

// wave is the one now coming, 1 to waves.
func (g *game) wave() int {
	if g.tick < firstWave {
		return 0
	}
	return min((g.tick-firstWave)/waveEvery+1, waves)
}

func (g *game) Status() (score, hint string) {
	watts := strconv.Itoa(g.watts) + string(wattSign)
	if g.wattsBlink > 0 && (g.wattsBlink/3)%2 == 1 {
		// Off, as the watts blink when they change: spaces as wide as the
		// figures, so that what is beside them does not move.
		watts = strings.Map(func(rune) rune { return blankFigure }, watts)
	}
	score = watts + "  WAVE " + strconv.Itoa(max(g.wave(), 1)) + "/" + strconv.Itoa(waves)
	switch {
	case g.over:
		hint = "SYSTEM DOWN - SPACE"
	case g.won:
		hint = "SECTOR CLEARED - SPACE"
	case g.dragging:
		hint = "DROP ON A TILE"
	case !g.started:
		hint = "SPACE OR CLICK TO BEGIN"
	case g.paused:
		hint = "PAUSED - SPACE"
	case g.message != "":
		hint = g.message
	case g.selected >= 0:
		hint = programs[g.selected].name + " - PICK A TILE"
	}
	return score, hint
}

// ---------------------------------------------------------------- acting

// place puts the program of a card on a tile, if the tile is free, the card
// ready and the watts enough; it says why not.
func (g *game) place(k Kind, c, r int) bool {
	g.message = ""
	switch {
	case c < 0 || r < 0 || c >= Cols || r >= Rows:
		return false
	case g.cells[r][c] != nil:
		g.message = "TILE TAKEN"
		return false
	case g.tick < g.cards[k].ready:
		g.message = "NOT READY"
		return false
	case g.watts < programs[k].cost:
		g.message = "NOT ENOUGH WATTS"
		return false
	}
	g.watts -= programs[k].cost
	g.wattsBlink = blinkFrames
	g.cards[k].ready = g.tick + programs[k].ready
	d := &defender{kind: k, hp: programs[k].hp, cd: programs[k].every}
	switch k {
	case Pulse, Cryo:
		d.cd = 20
	case Mine:
		d.armed = mineArms
	}
	g.cells[r][c] = d
	g.started = true
	return true
}

// collect takes the spark at index i.
func (g *game) collect(i int) {
	g.watts += g.sparks[i].value
	g.wattsBlink = blinkFrames
	owner := g.sparks[i].owner
	g.sparks = append(g.sparks[:i], g.sparks[i+1:]...)
	g.release(owner)
}

// release lets the dynamo that made a spark, taken or lost, make another.
func (g *game) release(owner int) {
	if owner <= 0 {
		return
	}
	if d := g.cells[(owner-1)/Cols][(owner-1)%Cols]; d != nil && d.kind == Dynamo {
		d.holding = false
	}
}

// sparkAt is the spark under a point of the screen, -1 for none: a spark is
// 10 pixels, and is taken from a little further.
func (g *game) sparkAt(px, py int) int {
	for i := len(g.sparks) - 1; i >= 0; i-- {
		s := g.sparks[i]
		if float64(px) >= s.x-4 && float64(px) < s.x+sparkSize+4 && float64(py) >= s.y-4 && float64(py) < s.y+sparkSize+4 {
			return i
		}
	}
	return -1
}

// sparkIn is a spark on a tile, -1 for none.
func (g *game) sparkIn(c, r int) int {
	for i, s := range g.sparks {
		cx, cy := int(s.x)+sparkSize/2-sectorX, int(s.y)+sparkSize/2-sectorY
		if cx >= c*TileW && cx < (c+1)*TileW && cy >= r*TileH && cy < (r+1)*TileH {
			return i
		}
	}
	return -1
}

func (g *game) Key(k games.Key) {
	switch {
	case g.over || g.won:
		if k == games.Action || k == games.Restart {
			g.best = max(g.best, g.wave())
			g.reset()
		}
		return
	case k == games.Left:
		g.curC = max(g.curC-1, 0)
	case k == games.Right:
		g.curC = min(g.curC+1, Cols-1)
	case k == games.Up:
		g.curR = max(g.curR-1, 0)
	case k == games.Down:
		g.curR = min(g.curR+1, Rows-1)
	case k == games.Restart:
		g.selected = (g.selected + 1) % int(kinds)
	case k == games.Action && !g.started && g.selected < 0:
		g.started = true
	case k == games.Action:
		if g.selected >= 0 {
			g.place(Kind(g.selected), g.curC, g.curR)
		} else if i := g.sparkIn(g.curC, g.curR); i >= 0 {
			g.collect(i)
		}
	}
	g.hoverC, g.hoverR = -1, -1
}

// Card and tile geometry, for the mouse.

func cardRect(i int) (x, y, w, h int) {
	return sectorX + i*(cardW+cardGap), cardsY, cardW, cardH
}

func (g *game) cardAt(px, py int) int {
	for i := 0; i < int(kinds); i++ {
		x, y, w, h := cardRect(i)
		if px >= x && py >= y && px < x+w && py < y+h {
			return i
		}
	}
	return -1
}

func tileAt(px, py int) (c, r int) {
	if px < sectorX || py < sectorY || px >= sectorX+Cols*TileW || py >= sectorY+Rows*TileH {
		return -1, -1
	}
	return (px - sectorX) / TileW, (py - sectorY) / TileH
}

// Pointer: over a card or a tile it hovers; a click that goes down and up on a
// card chooses it (or puts it down again), on a tile places the card in hand
// there, and on a spark, or on the tile it is in, takes it. A card can also be dragged to a tile, which
// places it where it is dropped; dropped off the sector, or let go of off the
// screen, it is not placed. Any click begins the game.
func (g *game) Pointer(p games.Pointer) {
	switch p.Kind {
	case games.PointerMove:
		if card := g.cardAt(p.X, p.Y); card != g.hoverCard {
			g.hoverCard, g.hoverTicks = card, 0
		}
		g.hoverC, g.hoverR = tileAt(p.X, p.Y)
		if g.downCard >= 0 && !g.over && !g.won {
			g.dragX, g.dragY = p.X, p.Y
			if dx, dy := p.X-g.downX, p.Y-g.downY; !g.dragging && dx*dx+dy*dy > dragSlop*dragSlop {
				g.dragging = true
			}
		}
	case games.PointerLeave:
		g.hoverCard, g.hoverC, g.hoverR, g.hoverTicks = -1, -1, -1, 0
		g.downCard, g.downSpark, g.dragging = -1, false, false
	case games.PointerDown:
		g.downCard = g.cardAt(p.X, p.Y)
		g.downX, g.downY, g.dragX, g.dragY, g.dragging = p.X, p.Y, p.X, p.Y, false
		g.downSpark = g.sparkAt(p.X, p.Y) >= 0
		if c, r := tileAt(p.X, p.Y); c >= 0 && g.sparkIn(c, r) >= 0 {
			g.downSpark = true
		}
		if g.over || g.won {
			return
		}
		// Sparks are taken as the button goes down, as it is quick to lose them: the
		// one that is under it, or those in the tile it is on, wherever in it.
		if !g.paused {
			if i := g.sparkAt(p.X, p.Y); i >= 0 {
				g.collect(i)
			}
			if c, r := tileAt(p.X, p.Y); c >= 0 {
				for i := g.sparkIn(c, r); i >= 0; i = g.sparkIn(c, r) {
					g.collect(i)
				}
			}
		}
	case games.PointerUp:
		if g.over || g.won {
			if g.cardAt(p.X, p.Y) < 0 {
				g.best = max(g.best, g.wave())
				g.reset()
			}
			return
		}
		down, spark, dragged := g.downCard, g.downSpark, g.dragging
		g.downCard, g.downSpark, g.dragging = -1, false, false
		if dragged && down >= 0 {
			if c, r := tileAt(p.X, p.Y); c >= 0 {
				g.curC, g.curR = c, r
				g.place(Kind(down), c, r)
			}
			return
		}
		switch i := g.cardAt(p.X, p.Y); {
		case i >= 0 && i == down:
			if g.selected == i {
				g.selected = -1
			} else {
				g.selected = i
			}
			g.message = ""
			g.started = true
		case !spark:
			if c, r := tileAt(p.X, p.Y); c >= 0 && g.selected >= 0 {
				g.curC, g.curR = c, r
				g.place(Kind(g.selected), c, r)
			}
		}
	}
}

// ------------------------------------------------------------------ steps

func (g *game) Step() {
	if !g.started || g.over || g.won || g.paused {
		return
	}
	g.tick++
	// Sparks fall from the sky now and then, in a tile of the sector.
	if g.tick >= g.nextSky {
		g.nextSky = g.tick + skySparkEvery
		g.sparks = append(g.sparks, spark{
			x: float64(sectorX + g.rng.Intn(Cols)*TileW + sparkOff), y: float64(sectorY + g.rng.Intn(Rows)*TileH + sparkOff),
			ttl: sparkLife, value: sparkValue})
	}
	for g.next < len(g.spawns) && g.spawns[g.next].at <= g.tick {
		s := g.spawns[g.next]
		g.next++
		g.enemies = append(g.enemies, &enemy{kind: s.kind, lane: s.lane, x: Cols, hp: malware[s.kind].hp})
	}
	g.stepDefenders()
	g.stepBolts()
	g.stepEnemies()
	live := g.sparks[:0]
	var lost []int
	for _, s := range g.sparks {
		if s.ttl--; s.ttl > 0 {
			live = append(live, s)
		} else {
			lost = append(lost, s.owner)
		}
	}
	g.sparks = live
	for _, owner := range lost {
		g.release(owner)
	}
	if g.next >= len(g.spawns) && len(g.enemies) == 0 && !g.over {
		g.won = true
		g.best = max(g.best, waves)
	}
}

// enemyAhead tells whether there is malware in a lane from a column to the right.
func (g *game) enemyAhead(lane, col int) bool {
	for _, e := range g.enemies {
		if e.lane == lane && e.x+0.8 > float64(col) {
			return true
		}
	}
	return false
}

func (g *game) stepDefenders() {
	for r := 0; r < Rows; r++ {
		for c := 0; c < Cols; c++ {
			d := g.cells[r][c]
			if d == nil {
				continue
			}
			switch d.kind {
			case Dynamo:
				// One that has made a spark waits for it to be taken, or lost.
				if d.holding {
					continue
				}
				if d.cd--; d.cd <= 0 {
					d.cd, d.holding = programs[Dynamo].every, true
					g.sparks = append(g.sparks, spark{x: float64(sectorX + c*TileW + sparkOff), y: float64(sectorY + r*TileH + sparkOff), ttl: sparkLife, value: sparkValue, owner: 1 + r*Cols + c})
				}
			case Pulse, Cryo:
				if d.cd > 0 {
					d.cd--
				}
				if d.cd <= 0 && g.enemyAhead(r, c) {
					d.cd = programs[d.kind].every
					g.bolts = append(g.bolts, bolt{lane: r, x: float64(c) + 0.8, cryo: d.kind == Cryo})
				}
			case Mine:
				if d.armed > 0 {
					d.armed--
					continue
				}
				for _, e := range g.enemies {
					if e.lane == r && e.x < float64(c)+1 && e.x+0.8 > float64(c) {
						g.blast(r, c)
						break
					}
				}
			}
		}
	}
}

// blast is a mine going off: it takes what is on its tile and the ones beside it.
func (g *game) blast(r, c int) {
	g.cells[r][c] = nil
	for _, e := range g.enemies {
		if e.lane == r && e.x < float64(c)+2 && e.x+0.8 > float64(c)-1 {
			e.hp -= mineBlast
		}
	}
	g.flash, g.flashLane = flashFrames, r
}

func (g *game) stepBolts() {
	live := g.bolts[:0]
	for _, b := range g.bolts {
		b.x += boltPace
		hit := false
		for _, e := range g.enemies {
			if e.lane == b.lane && b.x >= e.x && b.x <= e.x+0.8 && e.hp > 0 {
				if b.cryo {
					e.hp -= cryoDamage
					e.slow = slowSteps
				} else {
					e.hp -= boltDamage
				}
				hit = true
				break
			}
		}
		if !hit && b.x < Cols+1 {
			live = append(live, b)
		}
	}
	g.bolts = live
}

func (g *game) stepEnemies() {
	live := g.enemies[:0]
	for _, e := range g.enemies {
		if e.hp <= 0 {
			continue
		}
		m := malware[e.kind]
		pace := m.pace
		if e.slow > 0 {
			e.slow--
			pace /= 2
		}
		// The program in its way, if its front is in that tile.
		col := int(e.x)
		if col >= 0 && col < Cols && e.x > float64(col) && g.cells[e.lane][col] != nil {
			d := g.cells[e.lane][col]
			// A mine is stepped on, not eaten.
			if d.kind != Mine {
				if d.hp -= m.eats; d.hp <= 0 {
					g.cells[e.lane][col] = nil
				}
				live = append(live, e)
				continue
			}
		}
		e.x -= pace
		if e.x <= 0 {
			// It breaches the core, and is gone.
			g.core--
			g.shake = shakeFrames
			if g.core <= 0 {
				g.over = true
				g.best = max(g.best, g.wave())
			}
			continue
		}
		live = append(live, e)
	}
	g.enemies = live
}

// Key names, for the view.
func (e enemyKind) title() string { return malware[e].title }
