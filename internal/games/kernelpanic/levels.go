package kernelpanic

import "kero/internal/games"

// A group of malware in a wave.
type group struct {
	kind enemyKind
	n    int
}

// rewardNone is no program: the sector gives none.
const rewardNone = kinds

// A Level is a sector: which conduits are in play, what the player starts with,
// whether sparks fall from the sky, and the waves. Clearing it gives a program,
// which the player has from then on, and a mark of 1 to 3 chips by the core that
// was kept. Sectors go one after another, each opened by clearing the one before,
// and each brings something: a conduit, a program, a malware.
type Level struct {
	ID    string
	Lanes []int // the conduits in play, of Rows
	Watts int   // what the sector starts with
	Sky   bool  // sparks fall from the sky; if not, only dynamos make them
	// When the first wave comes, and how often the next, in steps.
	Start, Every int
	Waves        [][]group
	Reward       Kind // the program it gives, or rewardNone
}

// the malware of a level, once each, in the order of the kinds.
func (lv *Level) malware() []enemyKind {
	var seen [enemyKinds]bool
	for _, w := range lv.Waves {
		for _, gr := range w {
			seen[gr.kind] = true
		}
	}
	var out []enemyKind
	for k := enemyKind(0); k < enemyKinds; k++ {
		if seen[k] {
			out = append(out, k)
		}
	}
	return out
}

// total is how many malware a level has, and spans how many steps its waves do.
func (lv *Level) total() (n int) {
	for _, w := range lv.Waves {
		for _, gr := range w {
			n += gr.n
		}
	}
	return n
}

func (lv *Level) length() int { return lv.Start + len(lv.Waves)*lv.Every }

var all = []int{0, 1, 2, 3, 4}

// classic is the first game, in one sector: every conduit, every program, eight
// waves. It is the last sector, and what the tests play.
var classic = Level{
	ID: "", Lanes: all, Watts: startWatts, Sky: true, Start: firstWave, Every: waveEvery,
	Waves: [][]group{
		{{Drone, 2}},
		{{Drone, 3}},
		{{Drone, 3}, {Worm, 1}},
		{{Drone, 4}, {Worm, 2}},
		{{Trojan, 2}, {Drone, 3}},
		{{Trojan, 3}, {Worm, 3}},
		{{Trojan, 4}, {Drone, 5}},
		{{Trojan, 6}, {Drone, 6}, {Worm, 4}},
	},
	Reward: rewardNone,
}

// campaign is the sectors in order. The first has one conduit and the pulse
// alone; a sector brings a conduit, a program or a malware at a time.
var campaign = []Level{
	{ID: "1-1", Lanes: []int{2}, Watts: 150, Sky: true, Start: 10 * perSec, Every: 18 * perSec,
		Waves: [][]group{{{Drone, 2}}, {{Drone, 3}}}, Reward: Dynamo},
	{ID: "1-2", Lanes: []int{2}, Watts: 100, Sky: true, Start: 12 * perSec, Every: 20 * perSec,
		Waves: [][]group{{{Drone, 2}}, {{Drone, 3}}, {{Drone, 4}}}, Reward: Wall},
	{ID: "1-3", Lanes: []int{1, 2, 3}, Watts: 100, Sky: true, Start: 14 * perSec, Every: 22 * perSec,
		Waves: [][]group{{{Drone, 3}}, {{Drone, 3}, {Worm, 1}}, {{Drone, 4}, {Worm, 2}}}, Reward: Mine},
	// A blackout: no sparks from the sky, only the dynamos'.
	{ID: "1-4", Lanes: []int{1, 2, 3}, Watts: 175, Sky: false, Start: 14 * perSec, Every: 22 * perSec,
		Waves: [][]group{{{Drone, 3}}, {{Drone, 4}, {Worm, 1}}, {{Drone, 3}, {Worm, 3}}, {{Drone, 5}, {Worm, 2}}}, Reward: Cryo},
	{ID: "1-5", Lanes: all, Watts: 100, Sky: true, Start: 16 * perSec, Every: 24 * perSec,
		Waves: [][]group{{{Drone, 3}}, {{Drone, 4}, {Worm, 1}}, {{Trojan, 1}, {Drone, 3}}, {{Trojan, 1}, {Drone, 4}, {Worm, 2}}, {{Trojan, 2}, {Drone, 4}, {Worm, 3}}}, Reward: rewardNone},
	{ID: "1-6", Lanes: all, Watts: 100, Sky: true, Start: 16 * perSec, Every: 24 * perSec,
		Waves: [][]group{{{Drone, 4}}, {{Trojan, 1}, {Drone, 3}}, {{Trojan, 1}, {Worm, 3}}, {{Trojan, 2}, {Drone, 3}}, {{Trojan, 2}, {Drone, 4}, {Worm, 2}}, {{Trojan, 3}, {Drone, 4}, {Worm, 3}}}, Reward: rewardNone},
	{ID: "1-7", Lanes: all, Watts: 150, Sky: false, Start: 16 * perSec, Every: 24 * perSec,
		Waves: [][]group{{{Drone, 4}}, {{Drone, 3}, {Worm, 3}}, {{Trojan, 1}, {Drone, 4}}, {{Trojan, 2}, {Worm, 3}}, {{Trojan, 2}, {Drone, 4}, {Worm, 3}}, {{Trojan, 3}, {Drone, 5}, {Worm, 3}}}, Reward: rewardNone},
	{ID: "1-8", Lanes: all, Watts: 125, Sky: true, Start: 14 * perSec, Every: 22 * perSec,
		Waves: [][]group{{{Drone, 4}, {Worm, 2}}, {{Trojan, 1}, {Drone, 4}}, {{Trojan, 2}, {Worm, 3}}, {{Trojan, 2}, {Drone, 5}}, {{Trojan, 3}, {Worm, 4}}, {{Trojan, 3}, {Drone, 4}, {Worm, 3}}, {{Trojan, 3}, {Drone, 4}, {Worm, 4}}}, Reward: rewardNone},
	{ID: "1-9", Lanes: all, Watts: 150, Sky: false, Start: 14 * perSec, Every: 22 * perSec,
		Waves: [][]group{{{Drone, 5}}, {{Trojan, 1}, {Drone, 4}}, {{Trojan, 2}, {Worm, 3}}, {{Trojan, 2}, {Drone, 5}}, {{Trojan, 3}, {Worm, 3}}, {{Trojan, 3}, {Drone, 4}, {Worm, 3}}, {{Trojan, 4}, {Drone, 5}, {Worm, 4}}}, Reward: rewardNone},
	// The last: the classic waves, and a huge one after them.
	{ID: "1-10", Lanes: all, Watts: startWatts, Sky: true, Start: firstWave, Every: waveEvery,
		Waves:  append(append([][]group{}, classic.Waves[:len(classic.Waves)-1]...), classic.Waves[len(classic.Waves)-1], []group{{Trojan, 7}, {Drone, 7}, {Worm, 5}}),
		Reward: rewardNone},
}

// Progress is what the player has done: the best mark, in chips from 1 to 3, of
// each sector cleared.
type Progress struct {
	Chips map[string]int `json:"chips"`
}

// cleared tells whether a sector is.
func (p *Progress) cleared(id string) bool { return p.Chips[id] > 0 }

// unlocked tells whether a program is the player's: the pulse is from the start,
// and each of the others is the reward of a sector.
func (p *Progress) unlocked(k Kind) bool {
	if k == Pulse {
		return true
	}
	for i := range campaign {
		if campaign[i].Reward == k && p.cleared(campaign[i].ID) {
			return true
		}
	}
	return false
}

// open tells whether sector i can be played: the first always, and each when the
// one before it is cleared.
func (p *Progress) open(i int) bool {
	return i == 0 || (i > 0 && i < len(campaign) && p.cleared(campaign[i-1].ID))
}

// clear records a sector as cleared with a mark, keeping the best.
func (p *Progress) clear(id string, chips int) {
	if p.Chips == nil {
		p.Chips = map[string]int{}
	}
	p.Chips[id] = max(p.Chips[id], chips)
}

// next is the first sector that is not cleared, or the last.
func (p *Progress) next() int {
	for i := range campaign {
		if !p.cleared(campaign[i].ID) {
			return i
		}
	}
	return len(campaign) - 1
}

// A store keeps the progress between games.
type store interface {
	load() Progress
	save(Progress)
}

// fileStore is the progress in a file of Kero's, beside its configuration.
type fileStore struct{}

func (fileStore) load() (p Progress) {
	_ = games.Load("kernelpanic", &p)
	return p
}

func (fileStore) save(p Progress) { _ = games.Save("kernelpanic", p) }

// memStore keeps it as long as it is: for the games that are not saved.
type memStore struct{ p Progress }

func (m *memStore) load() Progress  { return m.p }
func (m *memStore) save(p Progress) { m.p = p }
