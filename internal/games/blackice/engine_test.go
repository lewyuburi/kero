package blackice

import (
	"strings"
	"testing"
	"time"
	"unicode"

	"kero/internal/games"
	"kero/internal/pixel"
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
		if x < 0 || y < sectorY+Rows*TileH || x+w > games.Width || y+h > infoY {
			t.Errorf("card %d at %d,%d %dx%d is off its place, under the sector", i, x, y, w, h)
		}
	}
	if sectorX < 0 || sectorX+Cols*TileW > games.Width || sectorY < games.HUDHeight || sectorY+Rows*TileH > games.Height {
		t.Error("the sector is off the screen")
	}
	// The core and the waves fit under the cards, on the screen.
	if infoY+9 > games.Height {
		t.Errorf("the core and the waves, at %d, are off the screen", infoY)
	}
	if c, r := tileAt(sectorX, sectorY); c != 0 || r != 0 {
		t.Errorf("the first tile is %d,%d", c, r)
	}
	if c, _ := tileAt(sectorX-1, sectorY); c != -1 {
		t.Error("a point left of the sector is on a tile")
	}
}

// The watts blink as they change, spent or gained, three times, and are shown
// when the blinking is over.
func TestWattsBlinkAsTheyChange(t *testing.T) {
	g := begun()
	g.nextSky = 1 << 30
	if g.Animating() || g.wattsBlink != 0 {
		t.Fatal("blinking with no change")
	}
	font := pixel.FontNamed("sans5x7")
	// The watts are shown when the score starts with a figure, and off when
	// it starts with the space of one.
	shown := func() bool {
		score, _ := g.Status()
		return unicode.IsDigit([]rune(score)[0])
	}
	g.place(Pulse, 0, 0) // spends 50
	if g.wattsBlink != blinkFrames || !g.Animating() {
		t.Fatalf("spending did not start the blink: %d", g.wattsBlink)
	}
	blanks, shows := 0, 0
	widths := map[int]bool{}
	for g.Animating() {
		if shown() {
			shows++
		} else {
			blanks++
		}
		score, _ := g.Status()
		widths[font.Width(score)] = true
		g.Frame()
	}
	if blanks < 6 || shows < 6 {
		t.Errorf("the watts were off for %d frames and on for %d: not blinking", blanks, shows)
	}
	if len(widths) != 1 {
		t.Errorf("the score changed its width while blinking, as pixels: %v", widths)
	}
	if !shown() {
		t.Error("the watts are off when the blink is over")
	}
	if score, _ := g.Status(); !strings.HasPrefix(score, "50"+string(wattSign)) {
		t.Errorf("score %q, want the 50 watts left, with the bolt", score)
	}
	// Gaining blinks too.
	g.sparks = []spark{{x: 10, y: 10, ttl: 100, value: 25}}
	g.collect(0)
	if g.wattsBlink != blinkFrames {
		t.Error("taking a spark did not start the blink")
	}
	// A card that cannot be placed spends nothing, and does not blink.
	g.wattsBlink = 0
	g.watts = 1
	g.place(Wall, 5, 5)
	g.place(Wall, 4, 4)
	if g.wattsBlink != 0 {
		t.Error("a placing that failed blinked")
	}
}

func ptr(g *game, k games.PointerKind, x, y int) {
	g.Pointer(games.Pointer{Kind: k, X: x, Y: y})
}

// A card dragged to a tile is placed where it is dropped; it is a drag when the
// pointer moves more than a few pixels, and not a click.
func TestDraggingACardToATile(t *testing.T) {
	g := newGame(1)
	cx, cy, cw, ch := cardRect(int(Pulse))
	fx, fy := cx+cw/2, cy+ch/2
	ptr(g, games.PointerMove, fx, fy)
	ptr(g, games.PointerDown, fx, fy)
	if g.dragging {
		t.Error("a press is a drag before it moves")
	}
	ptr(g, games.PointerMove, fx+1, fy+1)
	if g.dragging {
		t.Error("a move of a pixel is a drag")
	}
	tx, ty := centerOf(4, 1)
	ptr(g, games.PointerMove, fx+10, fy-30)
	if !g.dragging {
		t.Fatal("a move of many pixels is not a drag")
	}
	if _, hint := g.Status(); !strings.Contains(hint, "DROP") {
		t.Errorf("hint while dragging: %q", hint)
	}
	ptr(g, games.PointerMove, tx, ty)
	if g.hoverC != 4 || g.hoverR != 1 {
		t.Errorf("the drag is over tile %d,%d", g.hoverC, g.hoverR)
	}
	ptr(g, games.PointerUp, tx, ty)
	if d := g.cells[1][4]; d == nil || d.kind != Pulse {
		t.Fatalf("a pulse dropped on a tile was not placed: %+v", d)
	}
	if g.dragging || g.downCard != -1 {
		t.Error("the drag is not over after the drop")
	}
	if g.selected != -1 {
		t.Errorf("a drag chose card %d: it is not a click", g.selected)
	}
	if !g.started || g.watts != startWatts-50 {
		t.Errorf("started %v, watts %d", g.started, g.watts)
	}
}

func TestADragThatDoesNotLandIsNotPlaced(t *testing.T) {
	cx, cy, cw, ch := cardRect(int(Dynamo))
	fx, fy := cx+cw/2, cy+ch/2
	tx, ty := centerOf(2, 2)
	for name, drop := range map[string]func(g *game){
		"off the sector": func(g *game) { ptr(g, games.PointerUp, 3, games.Height-3) },
		"out of the screen": func(g *game) {
			ptr(g, games.PointerLeave, tx, ty)
			ptr(g, games.PointerUp, tx, ty) // a release that never comes from the pane
		},
	} {
		g := newGame(1)
		ptr(g, games.PointerDown, fx, fy)
		ptr(g, games.PointerMove, tx, ty)
		if !g.dragging {
			t.Fatalf("%s: no drag", name)
		}
		drop(g)
		for r := range g.cells {
			for c := range g.cells[r] {
				if g.cells[r][c] != nil {
					t.Errorf("%s: a program was placed at %d,%d", name, c, r)
				}
			}
		}
		if g.dragging || g.watts != startWatts {
			t.Errorf("%s: dragging %v, watts %d", name, g.dragging, g.watts)
		}
	}
	// Dropped on a tile that is taken, it says so and spends nothing.
	g := begun()
	g.cells[2][2] = &defender{kind: Wall, hp: 80}
	ptr(g, games.PointerDown, fx, fy)
	ptr(g, games.PointerMove, tx, ty)
	ptr(g, games.PointerUp, tx, ty)
	if g.message != "TILE TAKEN" || g.watts != startWatts {
		t.Errorf("a drop on a program: message %q, watts %d", g.message, g.watts)
	}
}

func TestAClickOnACardStillChoosesIt(t *testing.T) {
	g := newGame(1)
	cx, cy, cw, ch := cardRect(int(Cryo))
	ptr(g, games.PointerDown, cx+cw/2, cy+ch/2)
	ptr(g, games.PointerMove, cx+cw/2+1, cy+ch/2)
	ptr(g, games.PointerUp, cx+cw/2+1, cy+ch/2)
	if g.selected != int(Cryo) {
		t.Errorf("a click with a slip chose %d", g.selected)
	}
}

// The tooltip of a card shows after the pointer has stayed on it a moment, goes
// when it leaves or moves to another, and is not shown while a card is dragged;
// the game asks for frames only while it waits for it.
func TestCardTooltipShowsAfterAMoment(t *testing.T) {
	g := newGame(1)
	cx, cy, cw, ch := cardRect(int(Wall))
	ox, oy := cx+cw/2, cy+ch/2
	if g.tipShown() || g.Animating() {
		t.Fatal("a tooltip, or frames, with no pointer")
	}
	ptr(g, games.PointerMove, ox, oy)
	if g.tipShown() || !g.Animating() {
		t.Fatal("the tooltip is there at once, or no frames are asked for to wait")
	}
	for i := 0; i < tipDelay-1; i++ {
		g.Frame()
	}
	if g.tipShown() {
		t.Error("the tooltip shows before its time")
	}
	g.Frame()
	if !g.tipShown() {
		t.Fatal("the tooltip does not show after its time")
	}
	if g.Animating() {
		t.Error("frames are asked for while a tooltip stays")
	}
	// Moving within the card keeps it; to another card starts over.
	ptr(g, games.PointerMove, ox+3, oy-2)
	if !g.tipShown() {
		t.Error("moving within the card hid the tooltip")
	}
	nx, ny, nw, nh := cardRect(int(Mine))
	ptr(g, games.PointerMove, nx+nw/2, ny+nh/2)
	if g.tipShown() || g.hoverTicks != 0 {
		t.Error("another card kept the tooltip")
	}
	for i := 0; i < tipDelay; i++ {
		g.Frame()
	}
	ptr(g, games.PointerLeave, 0, 0)
	if g.tipShown() || g.Animating() {
		t.Error("the tooltip stayed after the pointer left")
	}
	// Dragging hides it.
	ptr(g, games.PointerMove, ox, oy)
	for i := 0; i < tipDelay; i++ {
		g.Frame()
	}
	ptr(g, games.PointerDown, ox, oy)
	ptr(g, games.PointerMove, ox+20, oy-20)
	if g.tipShown() {
		t.Error("the tooltip is there while dragging")
	}
}

// A click anywhere on the tile a spark is in takes it, not only on the spark;
// all those in the tile go together; and a click that took them does not also
// place a card there.
func TestClickOnTheTileTakesItsSparks(t *testing.T) {
	g := begun()
	g.nextSky = 1 << 30
	x, y := centerOf(3, 2)
	// The spark is in the tile's corner, far from where the click is.
	sx, sy := sectorX+3*TileW+sparkOff-4, sectorY+2*TileH+sparkOff-4
	g.sparks = []spark{{x: float64(sx), y: float64(sy), ttl: 100, value: 25}, {x: float64(sx + 3), y: float64(sy + 2), ttl: 100, value: 25}}
	cornerX, cornerY := sectorX+3*TileW+TileW-2, sectorY+2*TileH+TileH-2
	if g.sparkAt(cornerX, cornerY) >= 0 {
		t.Fatal("the click is on the spark itself: the test shows nothing")
	}
	w := g.watts
	g.Pointer(games.Pointer{Kind: games.PointerDown, X: cornerX, Y: cornerY})
	if g.watts != w+50 || len(g.sparks) != 0 {
		t.Errorf("a click on the tile took %d watts of its sparks, %d are left", g.watts-w, len(g.sparks))
	}
	// A spark in another tile is not taken.
	g.sparks = []spark{{x: float64(sectorX + 5*TileW + sparkOff), y: float64(sectorY + 1*TileH + sparkOff), ttl: 100, value: 25}}
	g.Pointer(games.Pointer{Kind: games.PointerDown, X: x, Y: y})
	if len(g.sparks) != 1 {
		t.Error("a click on a tile took a spark from another")
	}
	// With a card in hand, the click that takes a spark does not place it.
	g.selected = int(Pulse)
	g.sparks = []spark{{x: float64(sx), y: float64(sy), ttl: 100, value: 25}}
	g.Pointer(games.Pointer{Kind: games.PointerDown, X: cornerX, Y: cornerY})
	g.Pointer(games.Pointer{Kind: games.PointerUp, X: cornerX, Y: cornerY})
	if g.cells[2][3] != nil || len(g.sparks) != 0 {
		t.Errorf("the click that took the spark placed a card (%v) or left it (%d)", g.cells[2][3] != nil, len(g.sparks))
	}
	// And the next click on that tile places it.
	g.Pointer(games.Pointer{Kind: games.PointerDown, X: x, Y: y})
	g.Pointer(games.Pointer{Kind: games.PointerUp, X: x, Y: y})
	if g.cells[2][3] == nil {
		t.Error("with no spark in the tile, a click did not place the card")
	}
	// Paused, nothing is taken.
	g = begun()
	g.paused = true
	g.sparks = []spark{{x: float64(sx), y: float64(sy), ttl: 100, value: 25}}
	g.Pointer(games.Pointer{Kind: games.PointerDown, X: cornerX, Y: cornerY})
	if len(g.sparks) != 1 {
		t.Error("a spark was taken while paused")
	}
}

// A dynamo that is working goes through three frames of its bolt in a second,
// and again; once it has made a spark it is still, with a solid bolt, until the
// spark is taken.
func TestDynamoWorksInThreeFramesThenWaits(t *testing.T) {
	working := &defender{kind: Dynamo}
	var seen []string
	last := ""
	for tick := 0; tick < 3*perSec; tick++ {
		name := dynamoSprite(working, tick)
		if name != last {
			seen = append(seen, name)
			last = name
		}
	}
	want := []string{"dynamo-1", "dynamo-2", "dynamo-3"}
	if len(seen) != 9 {
		t.Fatalf("the bolt went through %v in 3 seconds, want three rounds of %v", seen, want)
	}
	for i, n := range seen {
		if n != want[i%3] {
			t.Errorf("frame %d is %s, want %s", i, n, want[i%3])
		}
	}
	if dynamoSprite(working, 0) != dynamoSprite(working, perSec) {
		t.Error("the frames do not repeat every second")
	}
	// Made its spark, it is still, whatever the step.
	holding := &defender{kind: Dynamo, holding: true}
	for tick := 0; tick < 2*perSec; tick++ {
		if dynamoSprite(holding, tick) != "dynamo" {
			t.Fatalf("a dynamo holding its spark is animated at step %d: %s", tick, dynamoSprite(holding, tick))
		}
	}
}

// A dynamo makes a spark and waits: it makes no other until that one is taken,
// or lost, and then it starts again.
func TestDynamoWaitsForItsSparkToBeTaken(t *testing.T) {
	g := begun()
	g.nextSky = 1 << 30
	g.place(Dynamo, 2, 1)
	d := g.cells[1][2]
	run(g, programs[Dynamo].every)
	if len(g.sparks) != 1 || !d.holding || g.sparks[0].owner != 1+1*Cols+2 {
		t.Fatalf("after its time: %d sparks, holding %v", len(g.sparks), d.holding)
	}
	// It waits: well past another cycle, and short of the spark's life, there is
	// no second spark.
	run(g, sparkLife-5)
	if len(g.sparks) != 1 {
		t.Fatalf("a dynamo holding a spark made another: %d sparks", len(g.sparks))
	}
	// Taken, it starts again, and makes its next after a whole cycle.
	g.collect(0)
	if d.holding {
		t.Fatal("a dynamo that was taken from is still holding")
	}
	run(g, programs[Dynamo].every-1)
	if len(g.sparks) != 0 {
		t.Error("a dynamo made a spark before its time was up")
	}
	run(g, 1)
	if len(g.sparks) != 1 || !d.holding {
		t.Errorf("a dynamo did not make its next spark after a cycle: %d", len(g.sparks))
	}
	// Lost, the same: it starts again.
	run(g, sparkLife)
	if len(g.sparks) != 0 || d.holding {
		t.Fatalf("a spark that was not taken stayed (%d) or its dynamo is still holding (%v)", len(g.sparks), d.holding)
	}
	run(g, programs[Dynamo].every)
	if len(g.sparks) != 1 {
		t.Errorf("a dynamo did not start again when its spark was lost: %d", len(g.sparks))
	}
	// A spark from the sky has no dynamo to release, and a dynamo that was eaten
	// has none either: neither breaks anything.
	g.sparks = []spark{{x: 50, y: 50, ttl: 3, value: 25}, {x: 60, y: 60, ttl: 100, value: 25, owner: 1 + 4*Cols + 8}}
	run(g, 3)
	g.collect(len(g.sparks) - 1)
	// Two dynamos are held independently.
	g = begun()
	g.nextSky = 1 << 30
	g.place(Dynamo, 0, 0)
	run(g, programs[Dynamo].ready)
	g.place(Dynamo, 0, 1)
	run(g, programs[Dynamo].every)
	if len(g.sparks) < 2 {
		t.Fatalf("two dynamos made %d sparks", len(g.sparks))
	}
	first := g.sparks[0]
	for i, s := range g.sparks {
		if s == first {
			g.collect(i)
			break
		}
	}
	held := 0
	for r := 0; r < 2; r++ {
		if g.cells[r][0].holding {
			held++
		}
	}
	if held != 1 {
		t.Errorf("taking one dynamo's spark released %d of the two", 2-held)
	}
}
