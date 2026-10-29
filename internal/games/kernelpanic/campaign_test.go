package kernelpanic

import (
	"strings"
	"testing"

	"kero/internal/games"
	"kero/internal/pixel"
)

// A campaign for a test, with progress that is its own, in memory.
func freshCampaign(chips map[string]int) (*game, *memStore) {
	st := &memStore{p: Progress{Chips: chips}}
	return newCampaign(1, st), st
}

// win plays the sector until the player has won it, with malware that dies at once.
func win(g *game) {
	g.started = true
	for i := 0; i < g.lv.length()+20*perSec && !g.won && !g.over; i++ {
		g.Step()
		for _, e := range g.enemies {
			e.hp = 0
		}
	}
}

func TestTheSectorsAreWellMade(t *testing.T) {
	ids := map[string]bool{}
	rewards := map[Kind]string{}
	for i, lv := range campaign {
		if lv.ID == "" || ids[lv.ID] {
			t.Errorf("sector %d has the id %q: none, or used", i, lv.ID)
		}
		ids[lv.ID] = true
		if len(lv.Lanes) == 0 || len(lv.Waves) == 0 || lv.Watts <= 0 || lv.Start <= 0 || lv.Every <= 0 {
			t.Errorf("sector %s is not made: %+v", lv.ID, lv)
		}
		seen := map[int]bool{}
		for _, r := range lv.Lanes {
			if r < 0 || r >= Rows || seen[r] {
				t.Errorf("sector %s has the conduit %d: off the sector, or twice", lv.ID, r)
			}
			seen[r] = true
		}
		for w, gr := range lv.Waves {
			if len(gr) == 0 {
				t.Errorf("sector %s wave %d is empty", lv.ID, w+1)
			}
			for _, g := range gr {
				if g.n <= 0 || g.kind >= enemyKinds {
					t.Errorf("sector %s wave %d has %+v", lv.ID, w+1, g)
				}
			}
		}
		if lv.Reward != rewardNone {
			if prev, dup := rewards[lv.Reward]; dup {
				t.Errorf("sector %s gives the %s that %s gave", lv.ID, lv.Reward.Name(), prev)
			}
			rewards[lv.Reward] = lv.ID
			if lv.Reward == Pulse {
				t.Errorf("sector %s gives the pulse, which the player has", lv.ID)
			}
		}
	}
	// Every program but the first is the reward of a sector, and it is won before
	// the sector that needs it.
	for k := Dynamo; k < kinds; k++ {
		if _, ok := rewards[k]; !ok {
			t.Errorf("no sector gives the %s", k.Name())
		}
	}
	// It begins with one conduit and ends with all of them, and the load grows.
	if len(campaign[0].Lanes) != 1 || len(campaign[len(campaign)-1].Lanes) != Rows {
		t.Errorf("the first has %d conduits, the last %d", len(campaign[0].Lanes), len(campaign[len(campaign)-1].Lanes))
	}
	if campaign[0].total() >= campaign[len(campaign)-1].total()/2 {
		t.Errorf("the first has %d malware and the last %d: no progress", campaign[0].total(), campaign[len(campaign)-1].total())
	}
	for i := 1; i < len(campaign); i++ {
		if len(campaign[i].Lanes) < len(campaign[i-1].Lanes) {
			t.Errorf("sector %s has fewer conduits than the one before", campaign[i].ID)
		}
	}
	// Trojans come after worms, and worms after drones.
	first := map[enemyKind]int{}
	for i := len(campaign) - 1; i >= 0; i-- {
		for _, k := range campaign[i].malware() {
			first[k] = i
		}
	}
	if !(first[Drone] <= first[Worm] && first[Worm] < first[Trojan]) {
		t.Errorf("first seen: drone %d, worm %d, trojan %d", first[Drone], first[Worm], first[Trojan])
	}
	// The last sector is the classic one's waves, and one more, the biggest.
	last := campaign[len(campaign)-1]
	if len(last.Waves) != len(classic.Waves)+1 || last.total() <= classic.total() {
		t.Errorf("the last sector has %d waves and %d malware; the classic %d and %d", len(last.Waves), last.total(), len(classic.Waves), classic.total())
	}
	for w := range classic.Waves {
		if len(last.Waves[w]) != len(classic.Waves[w]) {
			t.Errorf("wave %d of the last sector is not the classic one's", w+1)
		}
	}
}

func TestProgressOpensSectorsAndGivesPrograms(t *testing.T) {
	var p Progress
	if !p.open(0) || p.open(1) {
		t.Error("only the first sector is open at first")
	}
	if !p.unlocked(Pulse) || p.unlocked(Dynamo) || p.unlocked(Wall) {
		t.Error("only the pulse is the player's at first")
	}
	if p.next() != 0 {
		t.Errorf("next is %d", p.next())
	}
	p.clear("1-1", 2)
	p.clear("1-1", 1) // the best is kept
	if p.Chips["1-1"] != 2 || !p.open(1) || p.open(2) || !p.unlocked(Dynamo) || p.unlocked(Wall) {
		t.Errorf("after 1-1: %+v", p)
	}
	if p.next() != 1 {
		t.Errorf("next is %d", p.next())
	}
	for _, lv := range campaign {
		p.clear(lv.ID, 3)
	}
	for k := Kind(0); k < kinds; k++ {
		if !p.unlocked(k) {
			t.Errorf("%s is not the player's with everything cleared", k.Name())
		}
	}
	if p.next() != len(campaign)-1 || p.open(len(campaign)) || p.open(-1) {
		t.Errorf("the end: next %d", p.next())
	}
}

func TestACampaignStartsAtTheSectorsAndKeepsWhatIsDone(t *testing.T) {
	g, st := freshCampaign(nil)
	if g.screen != screenMenu || g.Interval() != 0 {
		t.Fatal("it does not start at the sectors, still")
	}
	if score, hint := g.Status(); score != "KERNEL PANIC" || !strings.Contains(hint, "PLAY") {
		t.Errorf("status at the sectors: %q %q", score, hint)
	}
	// A locked sector does not start.
	g.menuSel = 4
	g.Key(games.Action)
	if g.screen != screenMenu {
		t.Error("a sector that is locked started")
	}
	if _, hint := g.Status(); !strings.Contains(hint, "1-4") {
		t.Errorf("the hint of a locked sector says %q, not which to clear first", hint)
	}
	// The first starts, with one conduit and only the pulse.
	g.menuSel = 0
	g.Key(games.Action)
	if g.screen != screenPlay || g.lv.ID != "1-1" || g.watts != 150 {
		t.Fatalf("1-1 did not start: screen %v, %s, %d watts", g.screen, g.lv.ID, g.watts)
	}
	if !g.open[Pulse] || g.open[Dynamo] || g.open[Wall] || g.open[Mine] || g.open[Cryo] {
		t.Errorf("programs at 1-1: %v", g.open)
	}
	if !g.lanes[2] || g.lanes[0] || g.lanes[1] || g.lanes[3] || g.lanes[4] {
		t.Errorf("conduits at 1-1: %v", g.lanes)
	}
	// Winning it: marked by the core kept, a program given, and kept.
	win(g)
	if !g.won {
		t.Fatal("1-1 was not won")
	}
	if g.chips != 3 || g.reward != Dynamo {
		t.Errorf("1-1 won with chips %d and reward %v", g.chips, g.reward)
	}
	if st.p.Chips["1-1"] != 3 || !st.p.unlocked(Dynamo) {
		t.Errorf("what was done was not kept: %+v", st.p)
	}
	// The reward is shown, and then the sectors, with the next chosen and open.
	g.Key(games.Action)
	if g.screen != screenReward {
		t.Fatalf("after winning, screen %v, want the reward", g.screen)
	}
	if score, _ := g.Status(); score != "NEW PROGRAM" {
		t.Errorf("status at the reward: %q", score)
	}
	g.Key(games.Action)
	if g.screen != screenMenu || g.menuSel != 1 || !g.prog.open(1) {
		t.Errorf("after the reward: screen %v, chosen %d", g.screen, g.menuSel)
	}
	// And the next has what was given.
	g.Key(games.Action)
	if g.lv.ID != "1-2" || !g.open[Dynamo] || g.open[Wall] {
		t.Errorf("1-2: %s, programs %v", g.lv.ID, g.open)
	}
}

func TestAClearedSectorAgainGivesNothingNewAndKeepsTheBestMark(t *testing.T) {
	g, st := freshCampaign(map[string]int{"1-1": 3})
	g.startLevel(0)
	g.core = 1 // kept one chip
	win(g)
	if g.chips != 1 || g.reward != rewardNone {
		t.Errorf("again: chips %d, reward %v, want 1 and none", g.chips, g.reward)
	}
	if st.p.Chips["1-1"] != 3 {
		t.Errorf("the best mark was lost: %d", st.p.Chips["1-1"])
	}
	g.Key(games.Action)
	if g.screen != screenMenu {
		t.Errorf("with nothing new it goes to the sectors, not %v", g.screen)
	}
	// The marks follow the core: 3, 2, 1, and never none for a win.
	for core, want := range map[int]int{3: 3, 2: 2, 1: 1} {
		g, _ := freshCampaign(nil)
		g.startLevel(0)
		g.core = core
		win(g)
		if g.chips != want {
			t.Errorf("core %d gave %d chips, want %d", core, g.chips, want)
		}
	}
}

func TestLosingARetriesOrGoesToTheSectors(t *testing.T) {
	g, st := freshCampaign(nil)
	g.startLevel(0)
	g.started = true
	g.over = true
	if _, hint := g.Status(); !strings.Contains(hint, "RETRY") || !strings.Contains(hint, "MENU") {
		t.Errorf("hint after losing: %q", hint)
	}
	g.Key(games.Action)
	if g.screen != screenPlay || g.over || g.started || g.core != startCore {
		t.Errorf("space did not retry: screen %v, over %v", g.screen, g.over)
	}
	g.over = true
	g.Key(games.Restart)
	if g.screen != screenMenu {
		t.Errorf("enter after losing is %v, want the sectors", g.screen)
	}
	if len(st.p.Chips) != 0 {
		t.Errorf("losing marked a sector: %+v", st.p)
	}
	// M goes to the sectors from the play, too.
	g.startLevel(0)
	g.Key(games.Menu)
	if g.screen != screenMenu {
		t.Error("M did not go to the sectors")
	}
	// And a click retries, as space does.
	g.startLevel(0)
	g.over = true
	g.Pointer(games.Pointer{Kind: games.PointerDown, X: 100, Y: 100})
	g.Pointer(games.Pointer{Kind: games.PointerUp, X: 100, Y: 100})
	if g.over {
		t.Error("a click did not retry")
	}
}

func TestSectorsSetWhatIsInPlay(t *testing.T) {
	// A sector with one conduit: malware, and sparks, come only down it, and the
	// others are closed to programs.
	g, _ := freshCampaign(nil)
	g.startLevel(0)
	g.started = true
	for _, s := range g.spawns {
		if s.lane != 2 {
			t.Fatalf("malware in conduit %d, which is not in play", s.lane)
		}
	}
	if g.place(Pulse, 3, 0) || g.message != "CONDUIT CLOSED" {
		t.Errorf("a program in a closed conduit: placed, message %q", g.message)
	}
	if !g.place(Pulse, 3, 2) {
		t.Errorf("a program in the conduit in play was not placed: %q", g.message)
	}
	// A program the player has not got.
	g.cards[Wall].ready = 0
	if g.place(Wall, 5, 2) || g.message != "NOT YET YOURS" {
		t.Errorf("a program not the player's: message %q", g.message)
	}
	// Enter goes through the programs the player has, not the others.
	g.selected = -1
	for i := 0; i < 8; i++ {
		g.Key(games.Restart)
		if g.selected != int(Pulse) {
			t.Fatalf("enter chose card %d, with only the pulse the player's", g.selected)
		}
	}
	// Sparks from the sky fall in the conduit in play only.
	g.sparks, g.nextSky = nil, 1
	run(g, skySparkEvery*3)
	for _, s := range g.sparks {
		if _, r := tileAt(int(s.x)+sparkSize/2, int(s.y)+sparkSize/2); r != 2 {
			t.Errorf("a spark fell in conduit %d", r)
		}
	}
	// A blackout has no sparks from the sky, at all.
	g, _ = freshCampaign(map[string]int{"1-1": 3, "1-2": 3, "1-3": 3})
	g.startLevel(3)
	g.started = true
	g.watts = 0
	for i := 0; i < 3*skySparkEvery; i++ {
		g.Step()
		for _, e := range g.enemies {
			e.hp = 0
		}
	}
	if len(g.sparks) != 0 {
		t.Errorf("%d sparks from the sky in a blackout", len(g.sparks))
	}
	// A sector that is not a campaign's, the classic, has every program and conduit.
	c := newGame(1)
	for k := Kind(0); k < kinds; k++ {
		if !c.open[k] {
			t.Errorf("%s is not open in the classic sector", k.Name())
		}
	}
	for r := 0; r < Rows; r++ {
		if !c.lanes[r] {
			t.Errorf("conduit %d is closed in the classic sector", r)
		}
	}
}

func TestTheMenuTakesTheKeyboardAndTheMouse(t *testing.T) {
	g, _ := freshCampaign(map[string]int{"1-1": 2, "1-2": 1})
	if g.menuSel != 2 {
		t.Fatalf("the sector chosen at first is %d, want the next to play, 2", g.menuSel)
	}
	g.Key(games.Right)
	g.Key(games.Down)
	if g.menuSel != 3+menuCols {
		t.Errorf("right and down: %d", g.menuSel)
	}
	g.Key(games.Up)
	g.Key(games.Left)
	g.Key(games.Left)
	if g.menuSel != 1 {
		t.Errorf("up and left twice: %d", g.menuSel)
	}
	g.menuSel = 0
	g.Key(games.Left)
	if g.menuSel != len(campaign)-1 {
		t.Errorf("left from the first: %d", g.menuSel)
	}
	// The mouse: hovering chooses; a click goes down and up on a node to start it.
	x, y, w, h := nodeRect(1)
	g.Pointer(games.Pointer{Kind: games.PointerMove, X: x + w/2, Y: y + h/2})
	if g.menuSel != 1 || g.menuHov != 1 {
		t.Errorf("hover: chosen %d, over %d", g.menuSel, g.menuHov)
	}
	g.Pointer(games.Pointer{Kind: games.PointerDown, X: x + 3, Y: y + 3})
	g.Pointer(games.Pointer{Kind: games.PointerUp, X: x + 3, Y: y + 3})
	if g.screen != screenPlay || g.lv.ID != "1-2" {
		t.Fatalf("a click on 1-2 started %v %s", g.screen, g.lv.ID)
	}
	g.toMenu()
	// A drag from one node to another starts nothing; neither does a locked node.
	x2, y2, _, _ := nodeRect(0)
	g.Pointer(games.Pointer{Kind: games.PointerDown, X: x + 3, Y: y + 3})
	g.Pointer(games.Pointer{Kind: games.PointerUp, X: x2 + 3, Y: y2 + 3})
	g.Pointer(games.Pointer{Kind: games.PointerDown, X: x + 3, Y: y + 3})
	g.Pointer(games.Pointer{Kind: games.PointerLeave})
	g.Pointer(games.Pointer{Kind: games.PointerUp, X: x + 3, Y: y + 3})
	if g.screen != screenMenu {
		t.Error("a drag between nodes, or out, started a sector")
	}
	x5, y5, _, _ := nodeRect(5)
	g.Pointer(games.Pointer{Kind: games.PointerDown, X: x5 + 3, Y: y5 + 3})
	g.Pointer(games.Pointer{Kind: games.PointerUp, X: x5 + 3, Y: y5 + 3})
	if g.screen != screenMenu {
		t.Error("a locked sector started with a click")
	}
	// The reward goes on with a click.
	g.screen = screenReward
	g.Pointer(games.Pointer{Kind: games.PointerUp, X: 10, Y: 10})
	if g.screen != screenMenu {
		t.Error("a click did not go on from the reward")
	}
}

func TestTheMenuAndTheRewardAreDrawn(t *testing.T) {
	ink := func(cv *pixel.Canvas) (n int) {
		for y := 0; y < cv.H; y++ {
			for x := 0; x < cv.W; x++ {
				if cv.Get(x, y) {
					n++
				}
			}
		}
		return n
	}
	g, _ := freshCampaign(map[string]int{"1-1": 3, "1-2": 2})
	cv := pixel.NewCanvas(games.Width, games.Height)
	for sel := range campaign {
		g.menuSel = sel
		draw(g, cv)
		if ink(cv) < 2000 {
			t.Errorf("the sectors with %d chosen are nearly blank: %d pixels", sel, ink(cv))
		}
	}
	g.reward, g.screen = Cryo, screenReward
	for k := Dynamo; k < kinds; k++ {
		g.reward = k
		draw(g, cv)
		if ink(cv) < 400 {
			t.Errorf("the reward of %s is nearly blank", k.Name())
		}
	}
	// Nothing is drawn off the screen: the details fit under the nodes.
	if panelY+panelH > games.Height || menuX < 0 || menuX+panelW > games.Width {
		t.Errorf("the panel at %d, %dx%d is off the screen", panelY, panelW, panelH)
	}
	for i := range campaign {
		x, y, w, h := nodeRect(i)
		if x < 0 || y < games.HUDHeight || x+w > games.Width || y+h > panelY {
			t.Errorf("node %d at %d,%d is off its place", i, x, y)
		}
	}
	// A sector with closed conduits and locked cards draws too.
	g.screen = screenPlay
	g.startLevel(0)
	draw(g, cv)
	g.over = true
	draw(g, cv)
}

func TestTheGameKeepsProgressInAFile(t *testing.T) {
	t.Setenv("KERO_HOME", t.TempDir())
	a := newCampaign(1, fileStore{})
	a.startLevel(0)
	win(a)
	// Another game, as after Kero is opened again, finds what was done.
	b := newCampaign(2, fileStore{})
	if !b.prog.cleared("1-1") || !b.prog.unlocked(Dynamo) || b.menuSel != 1 {
		t.Errorf("a new game did not find the progress: %+v, chosen %d", b.prog, b.menuSel)
	}
	// The one that is not a campaign's keeps nothing in the file.
	n := newGame(3)
	win(n)
	c := newCampaign(4, fileStore{})
	if len(c.prog.Chips) != 1 {
		t.Errorf("the classic sector marked something: %+v", c.prog)
	}
}

// A conduit out of play has a lock where it begins and nothing else in it, and the
// one in play has the dots of its tiles and a line along each side: it is plain
// which is which.
func TestClosedConduitsHaveALockAndTheOneInPlayIsFramed(t *testing.T) {
	g, _ := freshCampaign(nil)
	g.startLevel(0)
	g.curC = Cols - 1 // the keyboard's corners are not at the start of the conduit
	cv := pixel.NewCanvas(games.Width, games.Height)
	draw(g, cv)
	inRect := func(r, x0, x1 int) (n int) {
		for y := sectorY + r*TileH + 2; y < sectorY+(r+1)*TileH-2; y++ {
			for x := x0; x < x1; x++ {
				if cv.Get(x, y) {
					n++
				}
			}
		}
		return n
	}
	for _, r := range []int{0, 1, 3, 4} {
		if n := inRect(r, sectorX, sectorX+TileW); n < 40 {
			t.Errorf("conduit %d, out of play, has %d inked pixels where its lock is: no lock", r, n)
		}
		if n := inRect(r, sectorX+TileW, sectorX+Cols*TileW); n != 0 {
			t.Errorf("conduit %d, out of play, has %d inked pixels past its lock: it is not plain", r, n)
		}
	}
	if n := inRect(2, sectorX+TileW, sectorX+Cols*TileW); n < Cols-2 {
		t.Errorf("the conduit in play has %d inked pixels: its tiles have no dots", n)
	}
	if n := inRect(2, sectorX, sectorX+TileW); n > 3 {
		t.Errorf("the conduit in play has a lock, or something: %d inked pixels at its start", n)
	}
	// Lines along both sides of the one in play, and not along a conduit out of play.
	for x := sectorX + TileW; x < sectorX+Cols*TileW; x += 17 {
		if !cv.Get(x, sectorY+2*TileH) || !cv.Get(x, sectorY+3*TileH-1) {
			t.Fatalf("the conduit in play is not framed at x %d", x)
		}
	}
	// With every conduit in play, there is no lock and no line inside the sector's own.
	full, _ := freshCampaign(map[string]int{"1-1": 3, "1-2": 3, "1-3": 3, "1-4": 3})
	full.startLevel(4)
	full.curC = Cols - 1
	c2 := pixel.NewCanvas(games.Width, games.Height)
	draw(full, c2)
	if c2.Get(sectorX+5, sectorY+1) {
		t.Error("a line inside the top of a sector with every conduit in play")
	}
	for r := 0; r < Rows; r++ {
		n := 0
		for y := sectorY + r*TileH + 2; y < sectorY+(r+1)*TileH-2; y++ {
			for x := sectorX; x < sectorX+TileW; x++ {
				if c2.Get(x, y) {
					n++
				}
			}
		}
		if n > 3 {
			t.Errorf("conduit %d of a sector with every one in play has a lock: %d pixels", r, n)
		}
	}
}

// Why a program cannot be placed is one thing, said by cannotPlace, which place
// and the view both go by.
func TestWhyAProgramCannotBePlaced(t *testing.T) {
	g, _ := freshCampaign(nil)
	g.startLevel(0)
	g.started = true
	for _, tc := range []struct {
		k    Kind
		c, r int
		want string
	}{
		{Pulse, 3, 2, ""},
		{Pulse, 3, 0, "CONDUIT CLOSED"},
		{Wall, 3, 2, "NOT YET YOURS"},
		{Pulse, -1, 2, "-"},
		{Pulse, 3, Rows, "-"},
	} {
		if got := g.cannotPlace(tc.k, tc.c, tc.r); got != tc.want {
			t.Errorf("%s at %d,%d: %q, want %q", tc.k.Name(), tc.c, tc.r, got, tc.want)
		}
	}
	g.place(Pulse, 3, 2)
	if got := g.cannotPlace(Pulse, 3, 2); got != "TILE TAKEN" {
		t.Errorf("on a program: %q", got)
	}
	if got := g.cannotPlace(Pulse, 4, 2); got != "NOT READY" {
		t.Errorf("a card not ready: %q", got)
	}
	g.cards[Pulse].ready = 0
	g.watts = 10
	if got := g.cannotPlace(Pulse, 4, 2); got != "NOT ENOUGH WATTS" {
		t.Errorf("without watts: %q", got)
	}
	// Asking changes nothing; placing says it, but says nothing of a tile off the sector.
	if g.watts != 10 || g.cells[2][4] != nil {
		t.Error("asking why not placed something or spent something")
	}
	g.message = "x"
	if g.place(Pulse, -3, 2) || g.message != "" {
		t.Errorf("a tile off the sector: message %q", g.message)
	}
}

// A card being dragged, or in hand, is dimmed where it cannot go, and almost solid
// where it can.
func TestAProgramIsDimmedWhereItCannotGo(t *testing.T) {
	ghost := func(g *game, c, r int) int {
		cv := pixel.NewCanvas(games.Width, games.Height)
		draw(g, cv)
		n := 0
		x, y := sectorX+c*TileW+spriteOff, sectorY+r*TileH+spriteOff
		for py := y; py < y+spriteSize; py++ {
			for px := x; px < x+spriteSize; px++ {
				if cv.Get(px, py) {
					n++
				}
			}
		}
		return n
	}
	g, _ := freshCampaign(nil)
	g.startLevel(0)
	g.started = true
	// In hand, over a tile of the conduit in play, and over a closed one.
	g.selected = int(Pulse)
	g.hoverC, g.hoverR = 4, 2
	okInHand := ghost(g, 4, 2)
	g.hoverC, g.hoverR = 4, 0
	closedInHand := ghost(g, 4, 0)
	if okInHand <= closedInHand+8 {
		t.Errorf("in hand, the ghost where it goes has %d inked pixels, and over a closed conduit %d: not dimmed", okInHand, closedInHand)
	}
	// Dragged: to a tile where it can go, snapped there; where it cannot, with the
	// pointer, dimmed.
	g.selected = -1
	g.dragging, g.downCard = true, int(Pulse)
	g.hoverC, g.hoverR = 5, 2
	good := ghost(g, 5, 2)
	if good < 40 {
		t.Errorf("dragged over a tile it can go on, the ghost has %d inked pixels there", good)
	}
	g.hoverC, g.hoverR = 5, 0 // closed
	g.dragX, g.dragY = sectorX+5*TileW+TileW/2, sectorY+0*TileH+TileH/2
	if refused := ghost(g, 5, 0); refused > good-20 {
		t.Errorf("dragged over a closed conduit, the ghost has %d inked pixels, against %d where it can go: not dimmed", refused, good)
	}
	// And off the sector, or over a tile that is taken, it is dimmed as well.
	g.cells[2][6] = &defender{kind: Wall, hp: 80}
	g.hoverC, g.hoverR = 6, 2
	g.dragX, g.dragY = sectorX+6*TileW+TileW/2, sectorY+2*TileH+TileH/2
	cv := pixel.NewCanvas(games.Width, games.Height)
	draw(g, cv)
	g.hoverC, g.hoverR = -1, -1
	g.dragX, g.dragY = 5, 5
	draw(g, cv)
}
