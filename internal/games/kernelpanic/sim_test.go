package kernelpanic

import (
	"fmt"
	"os"
	"testing"
)

// A bot plays a sector the way a person might: it takes every spark, builds an
// economy, then defends the conduits the malware comes down.
type bot struct {
	name    string
	dynamos int // how many dynamos it wants early
	pulses  int // pulses per conduit that has seen malware, at most
	walls   bool
	cryo    bool
	mines   bool
}

func (b bot) act(g *game, seen *[Rows]bool) {
	for len(g.sparks) > 0 {
		g.collect(0)
	}
	for _, e := range g.enemies {
		seen[e.lane] = true
	}
	lanes := g.lv.Lanes
	count := func(k Kind) (n int) {
		for r := 0; r < Rows; r++ {
			for c := 0; c < Cols; c++ {
				if d := g.cells[r][c]; d != nil && d.kind == k {
					n++
				}
			}
		}
		return n
	}
	try := func(k Kind, c, r int) bool {
		return g.open[k] && g.cells[r][c] == nil && g.place(k, c, r)
	}
	if count(Dynamo) < b.dynamos && g.open[Dynamo] {
		for _, r := range lanes {
			if try(Dynamo, 0, r) {
				return
			}
		}
	}
	for want := 1; want <= b.pulses; want++ {
		for _, r := range lanes {
			if !seen[r] && want == 1 && g.tick > g.lv.Start {
				continue
			}
			have := 0
			for c := 1; c <= 4; c++ {
				if d := g.cells[r][c]; d != nil && d.kind == Pulse {
					have++
				}
			}
			if have < want {
				for c := 1 + want; c >= 1; c-- {
					if try(Pulse, c, r) {
						return
					}
				}
			}
		}
	}
	if b.cryo && g.watts > 120 {
		for _, r := range lanes {
			if seen[r] && try(Cryo, 3, r) {
				return
			}
		}
	}
	if b.walls && g.watts > 100 {
		for _, r := range lanes {
			if seen[r] && try(Wall, 6, r) {
				return
			}
		}
	}
	if b.mines && g.watts > 60 {
		for _, r := range lanes {
			if seen[r] && try(Mine, 7, r) {
				return
			}
		}
	}
	if g.open[Dynamo] && g.watts > 150 && count(Dynamo) < b.dynamos+3 {
		for _, r := range lanes {
			if try(Dynamo, 0, r) {
				return
			}
		}
	}
}

// playSector plays sector i with everything before it cleared, and says whether it
// was won and with how much core.
func playSector(b bot, i int, seed int64) (won bool, core int) {
	st := &memStore{p: Progress{Chips: map[string]int{}}}
	for j := 0; j < i; j++ {
		st.p.clear(campaign[j].ID, 3)
	}
	g := newCampaign(seed, st)
	g.startLevel(i)
	g.started = true
	var seen [Rows]bool
	limit := g.lv.length() + 90*perSec
	for g.tick < limit && !g.over && !g.won {
		b.act(g, &seen)
		g.Step()
	}
	return g.won, g.core
}

var (
	careful   = bot{name: "careful", dynamos: 3, pulses: 2, walls: true}
	goodBot   = bot{name: "good", dynamos: 4, pulses: 2, walls: true, cryo: true, mines: true}
	expertBot = bot{name: "expert", dynamos: 5, pulses: 3, walls: true, cryo: true, mines: true}
)

// winRate is how many of seeds games a bot wins of sector i, in percent.
func winRate(b bot, i, seeds int) int {
	wins := 0
	for s := int64(1); s <= int64(seeds); s++ {
		if w, _ := playSector(b, i, s); w {
			wins++
		}
	}
	return wins * 100 / seeds
}

// The campaign is easy at first, and harder as it goes, and hard at the end: bots
// that play as a person might win the first sectors, and win fewer of the later
// ones, and the last is not given away to the best of them.
func TestTheCampaignGetsHarder(t *testing.T) {
	const seeds = 30
	good, expert := make([]int, len(campaign)), make([]int, len(campaign))
	for i := range campaign {
		good[i], expert[i] = winRate(goodBot, i, seeds), winRate(expertBot, i, seeds)
	}
	t.Logf("good   %v", good)
	t.Logf("expert %v", expert)
	for i := 0; i < 4; i++ {
		if good[i] < 95 {
			t.Errorf("sector %s is won by a good player only %d%% of the time: it should be the easy start", campaign[i].ID, good[i])
		}
	}
	for i := 0; i < 7; i++ {
		if expert[i] < 90 {
			t.Errorf("sector %s is won by an expert only %d%% of the time", campaign[i].ID, expert[i])
		}
	}
	// From the fifth on, it never gets much easier than the one before.
	for i := 5; i < len(campaign); i++ {
		if good[i] > good[i-1]+20 {
			t.Errorf("sector %s (%d%%) is a good deal easier than %s (%d%%)", campaign[i].ID, good[i], campaign[i-1].ID, good[i-1])
		}
	}
	last := len(campaign) - 1
	if expert[last] > 90 {
		t.Errorf("the last sector is won by an expert %d%% of the time: it is not the last", expert[last])
	}
	if good[last] > expert[last] || good[last-1] > expert[last-1]+10 {
		t.Errorf("a good player does better than an expert: %v against %v", good[last-1:], expert[last-1:])
	}
}

// A report of the same, with more games and a bot more: KP_SIM=1 go test -run Sim -v
func TestSimCampaign(t *testing.T) {
	if os.Getenv("KP_SIM") == "" {
		t.Skip("a balance report: KP_SIM=1")
	}
	bots := []bot{careful, goodBot, expertBot}
	const seeds = 120
	for _, b := range bots {
		fmt.Printf("%-8s", b.name)
		for i := range campaign {
			wins, core := 0, 0
			for s := int64(1); s <= seeds; s++ {
				w, c := playSector(b, i, s)
				if w {
					wins++
				}
				core += max(c, 0)
			}
			fmt.Printf(" %s:%3d%%/%.1f", campaign[i].ID, wins*100/seeds, float64(core)/seeds)
		}
		fmt.Println()
	}
}
