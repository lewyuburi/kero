// Package blackice is BLACK ICE, a game of defense by lanes: malware comes
// down five conduits of a fallen mainframe, and programs the player places
// hold it off. The engine here knows nothing of the window; see DESIGN.md for
// the game, and view.go for how it is drawn.
package blackice

import (
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"time"

	"kero/internal/games"
)

// The sector, in tiles, and where everything is on the screen (256 by 192).
const (
	Cols, Rows   = 9, 5
	TileW, TileH = 24, 24
	// The sector's top left, the cards' row above it, and the core below.
	sectorX, sectorY = (games.Width - Cols*TileW) / 2, 48
	cardsY           = 18
	cardW, cardH     = 40, 24
	cardGap          = 4
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
	blinkFrames   = 18 // the watts blink three times as they change
	flashFrames   = 6
)

type defender struct {
	kind  Kind
	hp    float64
	cd    int // steps to its next bolt or spark
	armed int // steps left before a mine is armed
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
	shake, flash   int
	wattsBlink     int
	flashLane      int
	best           int
	message        string // what the last try at placing said, for the hint
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
	g.downSpark = false
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

func (g *game) Animating() bool { return g.shake > 0 || g.flash > 0 || g.wattsBlink > 0 }

func (g *game) Frame() {
	g.shake, g.flash, g.wattsBlink = max(g.shake-1, 0), max(g.flash-1, 0), max(g.wattsBlink-1, 0)
}

// wave is the one now coming, 1 to waves.
func (g *game) wave() int {
	if g.tick < firstWave {
		return 0
	}
	return min((g.tick-firstWave)/waveEvery+1, waves)
}

func (g *game) Status() (score, hint string) {
	watts := strconv.Itoa(g.watts) + "W"
	if g.wattsBlink > 0 && (g.wattsBlink/3)%2 == 1 {
		// Off, as the watts blink when they change.
		watts = strings.Repeat(" ", len(watts))
	}
	score = watts + "  WAVE " + strconv.Itoa(max(g.wave(), 1)) + "/" + strconv.Itoa(waves)
	switch {
	case g.over:
		hint = "SYSTEM DOWN - SPACE"
	case g.won:
		hint = "SECTOR CLEARED - SPACE"
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
	g.sparks = append(g.sparks[:i], g.sparks[i+1:]...)
}

// sparkAt is the spark under a point of the screen, -1 for none: a spark is
// 8 pixels, and is taken from a little further.
func (g *game) sparkAt(px, py int) int {
	for i := len(g.sparks) - 1; i >= 0; i-- {
		s := g.sparks[i]
		if float64(px) >= s.x-4 && float64(px) < s.x+12 && float64(py) >= s.y-4 && float64(py) < s.y+12 {
			return i
		}
	}
	return -1
}

// sparkIn is a spark on a tile, -1 for none.
func (g *game) sparkIn(c, r int) int {
	for i, s := range g.sparks {
		cx, cy := int(s.x)+4-sectorX, int(s.y)+4-sectorY
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
// there, and on a spark takes it. Any click begins the game.
func (g *game) Pointer(p games.Pointer) {
	switch p.Kind {
	case games.PointerMove:
		g.hoverCard = g.cardAt(p.X, p.Y)
		g.hoverC, g.hoverR = tileAt(p.X, p.Y)
	case games.PointerLeave:
		g.hoverCard, g.hoverC, g.hoverR = -1, -1, -1
		g.downCard, g.downSpark = -1, false
	case games.PointerDown:
		g.downCard = g.cardAt(p.X, p.Y)
		g.downSpark = g.sparkAt(p.X, p.Y) >= 0
		if g.over || g.won {
			return
		}
		// A spark is taken as the button goes down: it is quick to lose.
		if i := g.sparkAt(p.X, p.Y); i >= 0 && !g.paused {
			g.collect(i)
		}
	case games.PointerUp:
		if g.over || g.won {
			if g.cardAt(p.X, p.Y) < 0 {
				g.best = max(g.best, g.wave())
				g.reset()
			}
			return
		}
		down, spark := g.downCard, g.downSpark
		g.downCard, g.downSpark = -1, false
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
			x: float64(sectorX + g.rng.Intn(Cols)*TileW + 8), y: float64(sectorY + g.rng.Intn(Rows)*TileH + 8),
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
	for _, s := range g.sparks {
		if s.ttl--; s.ttl > 0 {
			live = append(live, s)
		}
	}
	g.sparks = live
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
				if d.cd--; d.cd <= 0 {
					d.cd = programs[Dynamo].every
					g.sparks = append(g.sparks, spark{x: float64(sectorX + c*TileW + 8), y: float64(sectorY + r*TileH + 8), ttl: sparkLife, value: sparkValue})
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
