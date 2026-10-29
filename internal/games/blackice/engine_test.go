package blackice

import (
	"strings"
	"testing"
	"time"

	"kero/internal/games"
)

// run steps a game n times.
func run(g *game, n int) {
	for i := 0; i < n; i++ {
		g.Step()
	}
}

func begun() *game {
	g := newGame(1)
	g.started = true
	// No malware but what a test puts there, and one that never comes, so that
	// the sector is not cleared at the first step.
	g.spawns = []spawn{{at: 1 << 30}}
	return g
}

func TestAGameWaitsForThePlayer(t *testing.T) {
	g := newGame(1)
	if g.Interval() != 0 {
		t.Error("a game that has not begun has a clock")
	}
	g.Step()
	if g.tick != 0 {
		t.Error("a step went by before the game began")
	}
	g.Key(games.Action)
	if !g.started || g.Interval() != StepTime {
		t.Errorf("space did not begin it: started %v, interval %v", g.started, g.Interval())
	}
	if _, hint := newGame(1).Status(); !strings.Contains(hint, "BEGIN") {
		t.Errorf("hint before it begins: %q", hint)
	}
}

func TestPlacingCostsWattsAndWaitsForTheCard(t *testing.T) {
	g := begun()
	if !g.place(Pulse, 0, 0) || g.watts != startWatts-50 {
		t.Fatalf("a pulse was not placed, or did not cost 50: watts %d", g.watts)
	}
	if g.place(Pulse, 1, 0) || g.message != "NOT READY" {
		t.Errorf("a second pulse at once: message %q", g.message)
	}
	run(g, programs[Pulse].ready)
	if g.place(Pulse, 0, 0) || g.message != "TILE TAKEN" {
		t.Errorf("on a program: message %q", g.message)
	}
	g.watts = 10
	if g.place(Wall, 3, 1) || g.message != "NOT ENOUGH WATTS" {
		t.Errorf("without watts: message %q", g.message)
	}
	if g.place(Pulse, -1, 0) || g.place(Pulse, Cols, 0) || g.place(Pulse, 0, Rows) {
		t.Error("a program was placed off the sector")
	}
	g.watts = 100
	if !g.place(Pulse, 1, 0) {
		t.Errorf("a card that is ready, with the watts, was not placed: %q", g.message)
	}
}

func TestDynamosMakeSparksThatAreTaken(t *testing.T) {
	g := begun()
	g.nextSky = 1 << 30 // none from the sky
	g.place(Dynamo, 2, 1)
	run(g, programs[Dynamo].every)
	if len(g.sparks) != 1 {
		t.Fatalf("%d sparks after a dynamo's time, want 1", len(g.sparks))
	}
	w := g.watts
	g.Key(games.Right)
	g.Key(games.Right)
	g.Key(games.Up)
	if g.curC != 2 || g.curR != 1 {
		t.Fatalf("the cursor is at %d,%d", g.curC, g.curR)
	}
	g.Key(games.Action)
	if g.watts != w+sparkValue || len(g.sparks) != 0 {
		t.Errorf("a spark was not taken: watts %d, was %d, %d sparks", g.watts, w, len(g.sparks))
	}
	// One not taken is gone.
	g.sparks = []spark{{x: 100, y: 100, ttl: 3, value: 25}}
	run(g, 3)
	if len(g.sparks) != 0 {
		t.Error("a spark that was not taken stayed")
	}
}

func TestSparksFallFromTheSky(t *testing.T) {
	g := begun()
	run(g, skySparkEvery)
	if len(g.sparks) == 0 {
		t.Fatal("no spark from the sky")
	}
	s := g.sparks[0]
	if c, r := tileAt(int(s.x), int(s.y)); c < 0 || r < 0 {
		t.Errorf("a spark fell outside the sector: %v", s)
	}
}

func TestPulseKillsAndMalwareEats(t *testing.T) {
	g := begun()
	g.nextSky = 1 << 30
	g.place(Pulse, 0, 2)
	g.enemies = []*enemy{{kind: Drone, lane: 2, x: 6, hp: malware[Drone].hp}}
	for i := 0; i < 20*perSec && len(g.enemies) > 0; i++ {
		g.Step()
	}
	if len(g.enemies) != 0 {
		t.Fatalf("a pulse did not kill a drone alone in its lane: %+v", g.enemies[0])
	}
	if g.cells[2][0] == nil {
		t.Error("the drone ate the pulse before it died")
	}
	// A lane with no program is not fired down, and its malware gets through.
	g = begun()
	g.nextSky = 1 << 30
	g.place(Pulse, 0, 0)
	g.enemies = []*enemy{{kind: Drone, lane: 3, x: 4, hp: malware[Drone].hp}}
	run(g, 10*perSec)
	if g.enemies[0].hp != malware[Drone].hp {
		t.Error("a pulse fired down another lane")
	}
}

func TestMalwareEatsWhatIsInItsWayAndGoesOn(t *testing.T) {
	g := begun()
	g.nextSky = 1 << 30
	g.place(Dynamo, 4, 1)
	g.enemies = []*enemy{{kind: Drone, lane: 1, x: 5.1, hp: malware[Drone].hp}}
	run(g, 2*perSec)
	d := g.cells[1][4]
	if d == nil || d.hp >= programs[Dynamo].hp {
		t.Fatalf("the drone did not eat: %+v", d)
	}
	x := g.enemies[0].x
	run(g, perSec/2)
	if g.enemies[0].x != x {
		t.Error("it moved while it ate")
	}
	run(g, 3*perSec)
	if g.cells[1][4] != nil {
		t.Error("the dynamo survived being eaten")
	}
	x = g.enemies[0].x
	run(g, 3*perSec)
	if g.enemies[0].x >= x {
		t.Error("it did not go on after eating")
	}
}

func TestAWallTakesLongerThanADynamo(t *testing.T) {
	eaten := func(k Kind) int {
		g := begun()
		g.nextSky = 1 << 30
		g.place(k, 4, 1)
		g.enemies = []*enemy{{kind: Drone, lane: 1, x: 4.9, hp: 1e9}}
		for i := 1; i < 60*perSec; i++ {
			g.Step()
			if g.cells[1][4] == nil {
				return i
			}
		}
		return 60 * perSec
	}
	if w, d := eaten(Wall), eaten(Dynamo); w < 4*d {
		t.Errorf("a wall lasts %d steps and a dynamo %d: not much more", w, d)
	}
}

func TestACryoSlowsWhatItHits(t *testing.T) {
	g := begun()
	g.nextSky = 1 << 30
	g.place(Cryo, 0, 0)
	g.enemies = []*enemy{{kind: Worm, lane: 0, x: 3, hp: 1e9}}
	run(g, 3*perSec)
	if g.enemies[0].slow == 0 {
		t.Fatal("a cryo bolt did not slow a worm")
	}
	// Slowed, it goes half as fast: compare with one that is not.
	slowed := &enemy{kind: Worm, lane: 4, x: 5, hp: 1e9, slow: 1000}
	free := &enemy{kind: Worm, lane: 3, x: 5, hp: 1e9}
	g.enemies = []*enemy{slowed, free}
	run(g, 60)
	if d1, d2 := 5-slowed.x, 5-free.x; d1 < d2*0.45 || d1 > d2*0.55 {
		t.Errorf("slowed went %.3f tiles and free %.3f: not half", d1, d2)
	}
}

func TestAMineIsSteppedOnAndBlowsUp(t *testing.T) {
	g := begun()
	g.nextSky = 1 << 30
	g.place(Mine, 3, 2)
	// Not armed: it is stepped on and nothing happens.
	g.enemies = []*enemy{{kind: Trojan, lane: 2, x: 3.5, hp: malware[Trojan].hp}}
	g.Step()
	if g.cells[2][3] == nil || g.enemies[0].hp != malware[Trojan].hp {
		t.Error("a mine that is not armed went off, or hurt something")
	}
	g.enemies = nil
	run(g, mineArms)
	g.enemies = []*enemy{{kind: Trojan, lane: 2, x: 3.6, hp: malware[Trojan].hp}}
	g.Step()
	if g.cells[2][3] != nil {
		t.Error("an armed mine did not go off under malware")
	}
	if len(g.enemies) != 0 {
		t.Error("a mine did not kill what stepped on it")
	}
	if g.flash == 0 {
		t.Error("no flash from a mine")
	}
	// Another lane is safe.
	g = begun()
	g.nextSky = 1 << 30
	g.place(Mine, 3, 2)
	run(g, mineArms+1)
	g.enemies = []*enemy{{kind: Drone, lane: 0, x: 3.5, hp: 20}}
	g.Step()
	if g.cells[2][3] == nil || g.enemies[0].hp != 20 {
		t.Error("a mine went off for another lane")
	}
}

func TestBreachingTheCoreCostsChipsAndTheThirdLosesTheSector(t *testing.T) {
	g := begun()
	g.nextSky = 1 << 30
	for i := 0; i < startCore; i++ {
		g.enemies = append(g.enemies, &enemy{kind: Worm, lane: i, x: 0.005, hp: 12})
	}
	g.Step()
	if g.core != 0 || !g.over {
		t.Errorf("after three breaches: core %d, over %v", g.core, g.over)
	}
	if g.Interval() != 0 {
		t.Error("a lost game has a clock")
	}
	if g.shake == 0 || !g.Animating() {
		t.Error("a breach does not shake the screen")
	}
	if _, hint := g.Status(); !strings.Contains(hint, "DOWN") {
		t.Errorf("hint after losing: %q", hint)
	}
	g.Key(games.Action)
	if g.over || g.core != startCore || g.watts != startWatts || g.started {
		t.Errorf("space did not start it over: %+v", g)
	}

	g = begun()
	g.nextSky = 1 << 30
	g.enemies = []*enemy{{kind: Worm, lane: 0, x: 0.005, hp: 12}}
	g.Step()
	if g.core != startCore-1 || g.over || len(g.enemies) != 0 {
		t.Errorf("one breach: core %d, over %v, %d enemies", g.core, g.over, len(g.enemies))
	}
}

func TestTheWavesComeAndTheSectorIsCleared(t *testing.T) {
	g := newGame(3)
	if n := len(g.spawns); n != 2+3+4+6+5+6+9+16 {
		t.Errorf("%d malware in the waves", n)
	}
	for i := 1; i < len(g.spawns); i++ {
		if g.spawns[i].at < g.spawns[i-1].at {
			t.Fatal("the spawns are not in order")
		}
	}
	if g.spawns[0].at < firstWave {
		t.Errorf("the first comes at %d, before the first wave", g.spawns[0].at)
	}
	if last := g.spawns[len(g.spawns)-1].at; last >= firstWave+waves*waveEvery+10*perSec {
		t.Errorf("the last comes at %d", last)
	}
	kinds := map[enemyKind]bool{}
	for _, s := range g.spawns {
		kinds[s.kind] = true
		if s.lane < 0 || s.lane >= Rows {
			t.Errorf("a spawn in lane %d", s.lane)
		}
	}
	if len(kinds) != int(enemyKinds) {
		t.Error("not every kind of malware is in the waves")
	}
	// The same seed deals the same waves, another seed other.
	h, k := newGame(3), newGame(4)
	same, other := true, false
	for i := range g.spawns {
		if g.spawns[i] != h.spawns[i] {
			same = false
		}
		if g.spawns[i] != k.spawns[i] {
			other = true
		}
	}
	if !same || !other {
		t.Errorf("seeds: same %v, other %v", same, other)
	}
	// With malware that dies at once, the last wave being dead clears it.
	g.started = true
	g.nextSky = 1 << 30
	for i := 0; i < firstWave+waves*waveEvery+11*perSec && !g.won; i++ {
		g.Step()
		for _, e := range g.enemies {
			e.hp = 0
		}
	}
	if !g.won || g.Interval() != 0 {
		t.Errorf("the sector was not cleared: won %v at step %d", g.won, g.tick)
	}
	if g.wave() != waves {
		t.Errorf("wave %d at the end, want %d", g.wave(), waves)
	}
	if _, hint := g.Status(); !strings.Contains(hint, "CLEARED") {
		t.Errorf("hint after winning: %q", hint)
	}
}

func TestAPlayableGameIsNotLostAtOnce(t *testing.T) {
	// A defense of a pulse in each conduit and a few dynamos holds the first
	// waves: the game is hard, not impossible.
	g := newGame(5)
	g.started = true
	for r := 0; r < Rows; r++ {
		g.cells[r][0] = &defender{kind: Pulse, hp: programs[Pulse].hp, cd: 1}
		g.cells[r][1] = &defender{kind: Pulse, hp: programs[Pulse].hp, cd: 1}
	}
	run(g, firstWave+3*waveEvery)
	if g.over || g.core < startCore {
		t.Errorf("ten pulses lost to the first three waves: core %d at step %d", g.core, g.tick)
	}
}

func TestKeyboardPlays(t *testing.T) {
	g := newGame(1)
	g.Key(games.Restart) // chooses the first card
	if g.selected != 0 {
		t.Fatalf("enter chose card %d", g.selected)
	}
	for i := 0; i < int(kinds); i++ {
		g.Key(games.Restart)
	}
	if g.selected != 0 {
		t.Errorf("cards do not go round: %d", g.selected)
	}
	g.Key(games.Right)
	g.Key(games.Right)
	g.Key(games.Down)
	g.Key(games.Action)
	if g.cells[g.curR][2] == nil || g.cells[Rows/2+1][2] == nil || g.cells[Rows/2+1][2].kind != Pulse {
		t.Errorf("space did not place the pulse at the cursor %d,%d", g.curC, g.curR)
	}
	// The cursor stays in the sector.
	for i := 0; i < 20; i++ {
		g.Key(games.Left)
		g.Key(games.Up)
	}
	if g.curC != 0 || g.curR != 0 {
		t.Errorf("the cursor went to %d,%d", g.curC, g.curR)
	}
	for i := 0; i < 20; i++ {
		g.Key(games.Right)
		g.Key(games.Down)
	}
	if g.curC != Cols-1 || g.curR != Rows-1 {
		t.Errorf("the cursor went to %d,%d", g.curC, g.curR)
	}
}

// centerOf is a point of the screen in the middle of a tile.
func centerOf(c, r int) (int, int) {
	return sectorX + c*TileW + TileW/2, sectorY + r*TileH + TileH/2
}

func click(g *game, x, y int) {
	g.Pointer(games.Pointer{Kind: games.PointerMove, X: x, Y: y})
	g.Pointer(games.Pointer{Kind: games.PointerDown, X: x, Y: y})
	g.Pointer(games.Pointer{Kind: games.PointerUp, X: x, Y: y})
}

func TestMousePlaysCardThenTile(t *testing.T) {
	g := newGame(1)
	cx, cy, cw, ch := cardRect(int(Dynamo))
	click(g, cx+cw/2, cy+ch/2)
	if g.selected != int(Dynamo) {
		t.Fatalf("a click on the dynamo card chose %d", g.selected)
	}
	if !g.started {
		t.Error("choosing a card did not begin the game")
	}
	x, y := centerOf(3, 4)
	click(g, x, y)
	if d := g.cells[4][3]; d == nil || d.kind != Dynamo {
		t.Fatalf("a click on a tile did not place the dynamo: %+v", d)
	}
	// Clicking the card again puts it down.
	click(g, cx+cw/2, cy+ch/2)
	click(g, cx+cw/2, cy+ch/2)
	if g.selected != -1 && g.selected != int(Dynamo) {
		t.Errorf("selected %d", g.selected)
	}
	// With no card, a click on a tile does nothing.
	g.selected = -1
	x, y = centerOf(5, 0)
	click(g, x, y)
	if g.cells[0][5] != nil {
		t.Error("a click placed something with no card in hand")
	}
	// A drag from a card to a tile does not place it: a click goes down and up on one.
	g.selected = -1
	g.Pointer(games.Pointer{Kind: games.PointerDown, X: cx + 2, Y: cy + 2})
	x, y = centerOf(6, 0)
	g.Pointer(games.Pointer{Kind: games.PointerUp, X: x, Y: y})
	if g.selected != -1 || g.cells[0][6] != nil {
		t.Error("a drag from a card chose it or placed it")
	}
	// Hovering says where, and leaving says nowhere.
	g.Pointer(games.Pointer{Kind: games.PointerMove, X: x, Y: y})
	if g.hoverC != 6 || g.hoverR != 0 {
		t.Errorf("hover %d,%d", g.hoverC, g.hoverR)
	}
	g.Pointer(games.Pointer{Kind: games.PointerLeave})
	if g.hoverC != -1 || g.hoverCard != -1 {
		t.Error("hover stayed after the pointer left")
	}
}

func TestMouseTakesASparkAsItGoesDown(t *testing.T) {
	g := begun()
	g.nextSky = 1 << 30
	x, y := centerOf(2, 2)
	g.sparks = []spark{{x: float64(x - 4), y: float64(y - 4), ttl: 100, value: 25}}
	w := g.watts
	g.Pointer(games.Pointer{Kind: games.PointerDown, X: x, Y: y})
	if g.watts != w+25 || len(g.sparks) != 0 {
		t.Errorf("a spark was not taken as the button went down: watts %d", g.watts)
	}
	// And the same click does not also place a card there.
	g.selected = int(Pulse)
	g.sparks = []spark{{x: float64(x - 4), y: float64(y - 4), ttl: 100, value: 25}}
	g.Pointer(games.Pointer{Kind: games.PointerDown, X: x, Y: y})
	g.Pointer(games.Pointer{Kind: games.PointerUp, X: x, Y: y})
	if g.cells[2][2] != nil {
		t.Error("the click that took a spark also placed a card")
	}
}

func TestAClickRestartsAFinishedGame(t *testing.T) {
	g := begun()
	g.over = true
	g.Pointer(games.Pointer{Kind: games.PointerDown, X: 100, Y: 100})
	g.Pointer(games.Pointer{Kind: games.PointerUp, X: 100, Y: 100})
	if g.over || g.started {
		t.Error("a click did not start a lost game over")
	}
}

func TestEffectsRunOut(t *testing.T) {
	g := begun()
	g.shake, g.flash = shakeFrames, flashFrames
	for i := 0; i < shakeFrames; i++ {
		g.Frame()
	}
	if g.Animating() {
		t.Error("the effects did not end")
	}
}

func TestStepTimeIsAThirtiethOfASecond(t *testing.T) {
	if StepTime != time.Second/30 {
		t.Errorf("a step is %v", StepTime)
	}
}

func TestLayoutFitsTheScreen(t *testing.T) {
	for i := 0; i < int(kinds); i++ {
		x, y, w, h := cardRect(i)
		if x < 0 || y < games.HUDHeight || x+w > games.Width || y+h > sectorY {
			t.Errorf("card %d at %d,%d %dx%d is off its place", i, x, y, w, h)
		}
	}
	if sectorX < 0 || sectorX+Cols*TileW > games.Width || sectorY+Rows*TileH > games.Height {
		t.Error("the sector is off the screen")
	}
	if c, r := tileAt(sectorX, sectorY); c != 0 || r != 0 {
		t.Errorf("the first tile is %d,%d", c, r)
	}
	if c, _ := tileAt(sectorX-1, sectorY); c != -1 {
		t.Error("a point left of the sector is on a tile")
	}
}
