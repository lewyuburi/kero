package main

import (
	"math/rand"
	"time"
)

// gameKey is what a player can press, whatever the game.
type gameKey int

const (
	keyUp gameKey = iota
	keyDown
	keyLeft
	keyRight
	keyAction // space: pause, or begin
	keyRestart
)

// game is what a game pane plays: it steps at the interval it asks and takes
// keys. The engine knows nothing of the window, so it is tested without one;
// what a game looks like is its view's (gameKind.draw).
type game interface {
	// Interval is the time between steps, 0 while it waits for a key.
	Interval() time.Duration
	Step()
	Key(k gameKey)
	// Status is the score and a hint, which is empty while the game is on.
	Status() (score, hint string)
}

type point struct{ x, y int }

// bonusKind is a fruit that shows up now and then, and goes: each is worth
// something, and the clock slows the game down.
type bonusKind int

const (
	bonusCherry bonusKind = iota // points
	bonusStar                    // more points, but it is gone sooner
	bonusClock                   // slows the snake down for a while
)

// A bonus is on the board for ttl more steps.
type bonus struct {
	at   point
	kind bonusKind
	ttl  int
}

// The time a bonus stays, in steps, the points it is worth, and the chance
// in a hundred, each step with none on the board, that one shows up.
var bonusLife = map[bonusKind]int{bonusCherry: 45, bonusStar: 28, bonusClock: 40}
var bonusPoints = map[bonusKind]int{bonusCherry: 3, bonusStar: 6, bonusClock: 1}

const (
	bonusChance = 2.0 // percent
	slowSteps   = 30  // how long a clock slows it down
)

// snake is the game of the snake: it eats to grow, and dies on a wall or
// on itself.
type snake struct {
	w, h          int
	body          []point // the head first
	dir, queued   point
	food          point
	score, best   int
	started, over bool
	paused        bool
	bonus         *bonus
	slow          int // steps left of a clock's slowing
	rng           *rand.Rand
}

func newSnake(seed int64) *snake {
	g := &snake{w: snakeCols, h: snakeRows, rng: rand.New(rand.NewSource(seed))}
	g.reset()
	return g
}

func (g *snake) reset() {
	g.best = max(g.best, g.score)
	g.score, g.started, g.over, g.paused = 0, false, false, false
	mid := point{g.w / 2, g.h / 2}
	g.body = []point{mid, {mid.x - 1, mid.y}, {mid.x - 2, mid.y}}
	g.dir, g.queued = point{1, 0}, point{1, 0}
	g.bonus, g.slow = nil, 0
	g.placeFood()
}

func (g *snake) Interval() time.Duration {
	if !g.started || g.over || g.paused {
		return 0
	}
	// Faster as it grows, to a limit, and slower while a clock lasts.
	iv := max(60*time.Millisecond, 140*time.Millisecond-time.Duration(g.score)*3*time.Millisecond)
	if g.slow > 0 {
		iv = iv * 8 / 5
	}
	return iv
}

func (g *snake) Key(k gameKey) {
	turn := map[gameKey]point{keyUp: {0, -1}, keyDown: {0, 1}, keyLeft: {-1, 0}, keyRight: {1, 0}}
	switch d, isTurn := turn[k]; {
	case k == keyRestart && g.over:
		g.reset()
	case k == keyAction && g.over:
		g.reset()
	case k == keyAction && g.started:
		g.paused = !g.paused
	case k == keyAction:
		g.started = true
	case isTurn && !g.over:
		// Not back on itself.
		if d.x != -g.dir.x || d.y != -g.dir.y {
			g.queued = d
		}
		g.started, g.paused = true, false
	}
}

func (g *snake) Step() {
	if !g.started || g.over || g.paused {
		return
	}
	g.dir = g.queued
	head := point{g.body[0].x + g.dir.x, g.body[0].y + g.dir.y}
	eats := head == g.food
	took := g.bonus != nil && head == g.bonus.at
	// The tail moves out of the way, unless it eats.
	rest := g.body
	if !eats && !took {
		rest = g.body[:len(g.body)-1]
	}
	if head.x < 0 || head.y < 0 || head.x >= g.w || head.y >= g.h || contains(rest, head) {
		g.over = true
		g.best = max(g.best, g.score)
		return
	}
	g.body = append([]point{head}, rest...)
	if took {
		g.score += bonusPoints[g.bonus.kind]
		if g.bonus.kind == bonusClock {
			g.slow = slowSteps
		}
		g.bonus = nil
	}
	if eats {
		g.score++
		if len(g.body) == g.w*g.h {
			g.over = true
			g.best = max(g.best, g.score)
			return
		}
		g.placeFood()
	}
	g.tickBonus()
}

// tickBonus ages what the snake has not taken: a bonus goes as its time is
// up, and now and then one shows up, once the game has begun to be played.
func (g *snake) tickBonus() {
	if g.slow > 0 {
		g.slow--
	}
	if g.bonus != nil {
		if g.bonus.ttl--; g.bonus.ttl <= 0 {
			g.bonus = nil
		}
		return
	}
	if g.score < 1 || g.rng.Float64()*100 >= bonusChance {
		return
	}
	kinds := []bonusKind{bonusCherry, bonusCherry, bonusCherry, bonusClock, bonusClock, bonusStar}
	kind := kinds[g.rng.Intn(len(kinds))]
	if at, ok := g.freeCell(); ok {
		g.bonus = &bonus{at: at, kind: kind, ttl: bonusLife[kind]}
	}
}

func contains(ps []point, p point) bool {
	for _, q := range ps {
		if q == p {
			return true
		}
	}
	return false
}

// freeCell picks a cell that neither the snake nor what it eats is on.
func (g *snake) freeCell() (point, bool) {
	var free []point
	for y := 0; y < g.h; y++ {
		for x := 0; x < g.w; x++ {
			p := point{x, y}
			if contains(g.body, p) || p == g.food || (g.bonus != nil && p == g.bonus.at) {
				continue
			}
			free = append(free, p)
		}
	}
	if len(free) == 0 {
		return point{}, false
	}
	return free[g.rng.Intn(len(free))], true
}

// placeFood puts the food on a free cell.
func (g *snake) placeFood() {
	g.food = point{-1, -1}
	if at, ok := g.freeCell(); ok {
		g.food = at
	}
}

func (g *snake) Status() (score, hint string) {
	score = itoa(g.score) + "  ·  best " + itoa(max(g.best, g.score))
	switch {
	case g.over:
		hint = "game over · space to play again"
	case !g.started:
		hint = "arrows or WASD to begin"
	case g.paused:
		hint = "paused · space to go on"
	case g.slow > 0:
		hint = "slow"
	}
	return score, hint
}
