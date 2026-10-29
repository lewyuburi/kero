package kernelpanic

import "kero/internal/games"

// The sectors, as a menu of nodes in rows, and what happens at the end of one: it
// is cleared, and gives a program, or the system is down and it is tried again.

const (
	menuCols     = 5
	nodeW, nodeH = 50, 44
	nodeGap      = 10
	menuX        = (games.Width - (menuCols*nodeW + (menuCols-1)*nodeGap)) / 2
	menuY        = games.HUDHeight + 10
	panelY       = menuY + 2*nodeH + nodeGap + 12 // the details of the sector chosen
	panelH       = games.Height - panelY - 8
	panelW       = menuCols*nodeW + (menuCols-1)*nodeGap
)

// nodeRect is where the node of sector i is on the screen.
func nodeRect(i int) (x, y, w, h int) {
	return menuX + (i%menuCols)*(nodeW+nodeGap), menuY + (i/menuCols)*(nodeH+nodeGap), nodeW, nodeH
}

// nodeAt is the sector whose node is at a point of the screen, -1 for none.
func nodeAt(px, py int) int {
	for i := range campaign {
		x, y, w, h := nodeRect(i)
		if px >= x && py >= y && px < x+w && py < y+h {
			return i
		}
	}
	return -1
}

// menuHint is what is to be said under the sectors about the one chosen.
func (g *game) menuHint() string {
	if g.prog.open(g.menuSel) {
		return "ENTER TO PLAY"
	}
	return "CLEAR " + campaign[g.menuSel-1].ID + " FIRST"
}

// toMenu goes to the sectors, with the one to play next chosen.
func (g *game) toMenu() {
	g.screen = screenMenu
	g.menuSel = min(g.prog.next(), len(campaign)-1)
	g.menuHov, g.menuDown = -1, -1
}

func (g *game) menuKey(k games.Key) {
	n := len(campaign)
	switch k {
	case games.Left:
		g.menuSel = (g.menuSel + n - 1) % n
	case games.Right:
		g.menuSel = (g.menuSel + 1) % n
	case games.Up:
		if g.menuSel-menuCols >= 0 {
			g.menuSel -= menuCols
		}
	case games.Down:
		if g.menuSel+menuCols < n {
			g.menuSel += menuCols
		}
	case games.Action, games.Restart:
		if g.prog.open(g.menuSel) {
			g.startLevel(g.menuSel)
		}
	}
}

func (g *game) menuPointer(p games.Pointer) {
	i := nodeAt(p.X, p.Y)
	switch p.Kind {
	case games.PointerMove:
		g.menuHov = i
		if i >= 0 {
			g.menuSel = i
		}
	case games.PointerLeave:
		g.menuHov, g.menuDown = -1, -1
	case games.PointerDown:
		g.menuDown = i
	case games.PointerUp:
		down := g.menuDown
		g.menuDown = -1
		if i >= 0 && i == down {
			g.menuSel = i
			if g.prog.open(i) {
				g.startLevel(i)
			}
		}
	}
}

func (g *game) rewardKey(k games.Key) {
	if k == games.Action || k == games.Restart {
		g.toMenu()
	}
}

func (g *game) rewardPointer(p games.Pointer) {
	if p.Kind == games.PointerUp {
		g.toMenu()
	}
}

// finish is a sector of the campaign cleared: it is marked, by the core that was
// kept, 1 to 3 chips, the best of the times it is cleared; it gives its program,
// if it is the player's first time with it; and what was done is kept.
func (g *game) finish() {
	if !g.campaign {
		return
	}
	g.chips = max(1, min(g.core, 3))
	if r := g.lv.Reward; r != rewardNone && !g.prog.unlocked(r) {
		g.reward = r
	}
	g.prog.clear(g.lv.ID, g.chips)
	g.st.save(g.prog)
}

// after is a key at the end of a sector, won or lost: the sector lost is tried
// again, or the sectors are chosen from; the one cleared goes on to what it gave,
// or to the sectors. The classic sector alone is started over.
func (g *game) after(k games.Key) {
	if k != games.Action && k != games.Restart {
		return
	}
	switch {
	case !g.campaign:
		g.best = max(g.best, g.wave())
		g.reset()
	case g.won && g.reward != rewardNone:
		g.screen = screenReward
	case g.won:
		g.toMenu()
	case k == games.Restart:
		g.toMenu()
	default:
		g.startLevel(g.levelIdx)
	}
}
