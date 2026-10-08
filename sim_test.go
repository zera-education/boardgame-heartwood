package main

// Balance playtest: hundreds of World Tree games played by a team bot through
// the real rules engine (Apply, with the Keeper acting for each player and
// phones giving trust). Run with:
//
//	make sim   (HW_SIM=1 go test -run TestBalanceSim -v)
//
// The bot is a port of the team bot in docs/plan/world-tree/sim/engine.js:
// each player grows a Big Tree in their own value (or a value nobody stands
// for), harvests a fruit, explores Rings 1-2 for the treasures, walks onto the
// World Tree to place what they carry, and finally gathers there. Players with
// water to spare walk to a teammate who has run dry.

import (
	"fmt"
	"math/rand"
	"os"
	"strings"
	"testing"
)

const simCap = 40 // rounds before a game counts as stalled

type simStats struct {
	games, won, lost, stalled int
	rounds, actions           float64 // summed over won games
	shares                    map[string]float64
	doneTrees, doneFruit      float64 // the round each goal was reached, won games
	doneTreasure              float64
	sealedAtEnd, lowestWater  float64 // all games
}

var shareKinds = []string{"why", "ring", "harvest", "heartwood", "treasure"}

func TestBalanceSim(t *testing.T) {
	if os.Getenv("HW_SIM") == "" {
		t.Skip("set HW_SIM=1 to run the balance simulation")
	}
	const n = 300
	for _, players := range []int{6, 9, 12} {
		st := &simStats{shares: map[string]float64{}}
		for i := 0; i < n; i++ {
			simGame(t, players, st)
		}
		simReport(t, players, st)
	}
}

func simReport(t *testing.T, players int, s *simStats) {
	n := float64(s.games)
	w := float64(max(s.won, 1))
	var b strings.Builder
	fmt.Fprintf(&b, "\n=== %d players, %d games (cap %d rounds)\n", players, s.games, simCap)
	fmt.Fprintf(&b, "won %.0f%%   lost (everyone dry) %.0f%%   stalled %.0f%%\n",
		100*float64(s.won)/n, 100*float64(s.lost)/n, 100*float64(s.stalled)/n)
	fmt.Fprintf(&b, "won games: %.1f rounds, %.0f actions\n", s.rounds/w, s.actions/w)
	total := 0.0
	var parts []string
	for _, k := range shareKinds {
		total += s.shares[k] / w
		parts = append(parts, fmt.Sprintf("%s %.1f", k, s.shares[k]/w))
	}
	fmt.Fprintf(&b, "shares per won game: %.1f (%s)\n", total, strings.Join(parts, ", "))
	fmt.Fprintf(&b, "goal reached by round (won games): trees %.1f, fruit %.1f, treasures %.1f\n",
		s.doneTrees/w, s.doneFruit/w, s.doneTreasure/w)
	fmt.Fprintf(&b, "rough length: %.0f minutes (1.5 min a share, 1.5 a round, 15 s an action)\n",
		total*1.5+s.rounds/w*1.5+s.actions/w*0.25)
	fmt.Fprintf(&b, "all games: sealed hexes at the end %.1f, lowest water seen %.2f\n", s.sealedAtEnd/n, s.lowestWater/n)
	t.Log(b.String())
}

// bot plays one game for everyone.
type bot struct {
	g      *Game
	shares map[string]float64
}

func simGame(t *testing.T, players int, st *simStats) {
	g := NewGame("SIM", "host")
	for i := 0; i < players; i++ {
		p, err := g.Join(fmt.Sprintf("P%d", i), Colors[i])
		if err != nil {
			t.Fatal(err)
		}
		// One type in turn (1..9), plus one other at random.
		types := []int{i%9 + 1}
		for len(types) < 2 {
			if ty := 1 + rand.Intn(9); ty != types[0] {
				types = append(types, ty)
			}
		}
		if err := g.Apply(Action{Host: "host", Type: "assignPowers", Target: p.ID, Types: types}); err != nil {
			t.Fatal(err)
		}
	}
	for _, ty := range []string{"start", "begin"} {
		if err := g.Apply(Action{Host: "host", Type: ty}); err != nil {
			t.Fatal(err)
		}
	}
	b := &bot{g: g, shares: map[string]float64{}}
	for g.Phase == PhaseEnter {
		v := g.TurnIdx % 6
		spots := ringHexes(4, v)
		if !b.act(g.current(), "enter", Action{N: v, Hex: spots[rand.Intn(len(spots))]}) {
			t.Fatal("bot could not enter")
		}
	}
	at := map[string]int{}
	lowest, actions := 99, 0
	for g.Phase == PhaseTurn && g.Round <= simCap {
		p := g.current()
		actions += b.turn(p)
		for _, q := range g.Players {
			lowest = min(lowest, q.Water)
		}
		o := g.goals()
		done := map[string]bool{"trees": !slicesContainsFalse(o.Trees[:]), "fruit": o.PlacedPlayers == o.Players,
			"treasure": len(o.Treasures) == len(Treasures)}
		for k, ok := range done {
			if _, seen := at[k]; ok && !seen {
				at[k] = g.Round
			}
		}
	}
	st.games++
	switch g.Result {
	case "won":
		st.won++
		st.rounds += float64(g.Round)
		st.actions += float64(actions)
		for k, v := range b.shares {
			st.shares[k] += v
		}
		st.doneTrees += float64(at["trees"])
		st.doneFruit += float64(at["fruit"])
		st.doneTreasure += float64(at["treasure"])
	case "lost":
		st.lost++
	default:
		st.stalled++
	}
	for _, tl := range g.Tiles {
		if tl != nil && tl.Leaves >= 2 {
			st.sealedAtEnd++
		}
	}
	st.lowestWater += float64(lowest)
}

func slicesContainsFalse(bs []bool) bool {
	for _, b := range bs {
		if !b {
			return true
		}
	}
	return false
}

// act is the Keeper tapping an action for p, then every share it opened being
// told: listeners give a trust acorn now and then, and the Keeper taps Done.
func (b *bot) act(p *Player, typ string, a Action) bool {
	if typ != "enter" && typ != "move" && typ != "pass" && typ != "explore" && a.Hex == 0 {
		a.Hex = -1
	}
	a.Host, a.As, a.Type = "host", p.ID, typ
	if err := b.g.Apply(a); err != nil {
		return false
	}
	for len(b.g.Shares) > 0 {
		s := b.g.Shares[0]
		b.shares[s.Kind]++
		if s.Kind != "treasure" {
			for _, q := range b.g.Players {
				if q.ID != s.Player && rand.Intn(10) < 4 {
					b.g.Apply(Action{Pid: q.ID, Secret: q.Secret, Type: "trust"})
				}
			}
		}
		b.g.Apply(Action{Host: "host", Type: "doneShare"})
	}
	return true
}

// turn plays p's turn and returns the actions spent.
func (b *bot) turn(p *Player) int {
	g := b.g
	spent := 0
	for guard := 1; g.Phase == PhaseTurn && g.Turn.Player == p.ID && guard <= 12; guard++ {
		before := g.Turn.Actions
		if !b.step(p) || g.Phase != PhaseTurn {
			break
		}
		spent += before - g.Turn.Actions
		if g.Turn.Actions == before && guard > 8 {
			break
		}
	}
	if g.Phase == PhaseTurn {
		b.act(p, "endTurn", Action{})
	}
	return spent
}

// path finds a route from `from` to a hex where goal holds, through hexes p may
// enter (or ignoring dead leaves, to find what to clear). It returns the hexes
// after `from`, and false when there is no route.
func (b *bot) path(p *Player, from int, goal func(int) bool, ignoreLeaves bool) ([]int, bool) {
	prev := map[int]int{from: -1}
	q := []int{from}
	for len(q) > 0 {
		i := q[0]
		q = q[1:]
		if goal(i) {
			var out []int
			for k := i; k != from; k = prev[k] {
				out = append([]int{k}, out...)
			}
			return out, true
		}
		for _, j := range Adj[i] {
			if _, seen := prev[j]; !seen && (ignoreLeaves || b.g.canEnter(p, j)) {
				prev[j] = i
				q = append(q, j)
			}
		}
	}
	return nil, false
}

func (b *bot) goToward(p *Player, goal func(int) bool) bool {
	route, ok := b.path(p, p.Pos, goal, false)
	if ok && len(route) > 0 {
		return b.act(p, "move", Action{Hex: route[0]})
	}
	if ok {
		return false
	}
	r, ok := b.path(p, p.Pos, goal, true)
	if !ok || len(r) == 0 {
		return false
	}
	// Blocked by dead leaves: clear the first sealed hex on the way.
	if b.g.sealed(r[0]) && !b.g.canEnter(p, r[0]) {
		return b.act(p, "clear", Action{Hex: r[0]})
	}
	if b.g.canEnter(p, r[0]) {
		return b.act(p, "move", Action{Hex: r[0]})
	}
	return false
}

func is(h int) func(int) bool { return func(i int) bool { return i == h } }

func (b *bot) any(f func(int) bool) bool {
	for i := 1; i < len(Board); i++ {
		if f(i) {
			return true
		}
	}
	return false
}

// step is one action (or a free pass); false when the bot has nothing useful to do.
func (b *bot) step(p *Player) bool {
	g := b.g
	t := g.tile(p.Pos)
	o := g.goals()
	allFruit := o.PlacedPlayers == o.Players
	// Free: water for a dry teammate in reach; a spare fruit for someone who needs one.
	for _, q := range g.Players {
		if q != p && q.Water == 0 && p.Water >= 2 && g.canPass(p, q) {
			return b.act(p, "pass", Action{Target: q.ID, Text: "water"})
		}
	}
	if p.Placed > 0 && p.Fruit > 0 {
		for _, q := range g.Players {
			if q != p && q.Placed == 0 && q.Fruit == 0 && g.canPass(p, q) {
				return b.act(p, "pass", Action{Target: q.ID, Text: "fruit"})
			}
		}
	}
	if g.Turn.Actions < 1 {
		return false
	}
	// 0. A teammate is out of water: the nearest one who can spare it walks over.
	var dry *Player
	for _, q := range g.Players {
		if q != p && q.Water == 0 {
			dry = q
			break
		}
	}
	if dry != nil && p.Water >= 3 {
		var best *Player
		bestD := 0
		for _, q := range g.Players {
			if q == dry || q.Water < 3 {
				continue
			}
			d := 99
			if r, ok := b.path(q, q.Pos, is(dry.Pos), false); ok {
				d = len(r)
			}
			if best == nil || d < bestD {
				best, bestD = q, d
			}
		}
		if best == p && b.goToward(p, is(dry.Pos)) {
			return true
		}
	}
	// 1. Water low: find a spring.
	if p.Water <= 1 {
		if t != nil && t.Up && t.Kind == "spring" {
			return b.act(p, "drink", Action{})
		}
		for i, x := range g.Tiles {
			if x != nil && x.Up && x.Kind == "spring" {
				if p.Water > 0 && b.goToward(p, is(i)) {
					return true
				}
				break
			}
		}
	}
	// 2. Carrying a treasure or my first fruit: walk onto the World Tree.
	if p.Treasure != "" || (p.Fruit > 0 && p.Placed == 0) {
		return b.goToward(p, is(0))
	}
	// 2b. Carrying a spare fruit: walk to someone who still needs one.
	if p.Placed > 0 && p.Fruit > 0 && !allFruit {
		needy := func(i int) bool {
			for _, q := range g.Players {
				if q.Pos == i && q.Placed == 0 && q.Fruit == 0 {
					return true
				}
			}
			return false
		}
		if b.goToward(p, needy) {
			return true
		}
	}
	// 3. A treasure lies here: take it.
	if t != nil && t.Up && t.Treasure != "" {
		return b.act(p, "take", Action{})
	}
	// 4. Grow a Big Tree in my value (or a value nobody stands for).
	target := -1
	if !o.Trees[p.Value] {
		target = p.Value
	} else {
		for s := range 6 {
			taken := false
			for _, q := range g.Players {
				taken = taken || q.Value == s
			}
			if !o.Trees[s] && !taken {
				target = s
				break
			}
		}
	}
	if target >= 0 {
		inT := func(i int) bool { return i > 0 && Board[i].Sector == target }
		if inT(p.Pos) && t.Leaves < 2 {
			if t.Stage == 3 {
				return b.act(p, "tend", Action{})
			}
			if (t.Stage == 1 || t.Stage == 2) && p.Water >= 2 && g.wx(p.Pos) != "rain" {
				return b.act(p, "water", Action{})
			}
			if sowable(t) && !b.any(func(i int) bool { return inT(i) && g.Tiles[i].Stage > 0 && g.Tiles[i].Leaves < 2 }) {
				return b.act(p, "sow", Action{})
			}
			if !t.Up && !b.any(func(i int) bool {
				x := g.Tiles[i]
				return inT(i) && (x.Stage > 0 || sowable(x)) && x.Leaves < 2
			}) {
				return b.act(p, "explore", Action{})
			}
		}
		growing := func(i int) bool {
			return inT(i) && g.Tiles[i].Stage > 0 && g.Tiles[i].Stage < 4 && g.Tiles[i].Leaves < 2
		}
		waiting := growing(p.Pos) && (g.wx(p.Pos) == "rain" || p.Water < 2) // the rain grows it, or I need water first
		if !waiting {
			goal := func(i int) bool { return inT(i) && !g.Tiles[i].Up && g.Tiles[i].Leaves < 2 }
			if b.any(growing) {
				goal = growing
			} else if b.any(func(i int) bool { return inT(i) && sowable(g.Tiles[i]) }) {
				goal = func(i int) bool { return inT(i) && sowable(g.Tiles[i]) }
			}
			if !goal(p.Pos) && b.goToward(p, goal) {
				return true
			}
		}
	}
	// 5. Fruit: harvest at a Big Tree out of the rain.
	if (p.Placed == 0 && p.Fruit == 0) || (p.Placed > 0 && !allFruit && p.Fruit < 1) {
		ripe := func(i int) bool {
			x := g.tile(i)
			return x != nil && x.Stage == 4 && x.Leaves < 2 && !x.Harvested && g.wx(i) != "rain"
		}
		if ripe(p.Pos) {
			return b.act(p, "harvest", Action{})
		}
		goal := func(i int) bool { x := g.tile(i); return x != nil && x.Stage == 4 && x.Leaves < 2 }
		if b.any(ripe) {
			goal = ripe
		}
		if b.goToward(p, goal) {
			return true
		}
	}
	// 6. Treasures: take a loose one, or explore Rings 1-2 for the hidden ones.
	hidden, loose, carried := 0, -1, 0
	for i, x := range g.Tiles {
		if x == nil {
			continue
		}
		if x.Kind == "treasure" && !x.Up {
			hidden++
		}
		if x.Up && x.Treasure != "" && loose < 0 {
			loose = i
		}
	}
	for _, q := range g.Players {
		if q.Treasure != "" {
			carried++
		}
	}
	if loose > 0 && p.Treasure == "" {
		return b.goToward(p, is(loose))
	}
	if hidden > 0 && len(g.Placed)+carried < len(Treasures) {
		if t != nil && !t.Up {
			return b.act(p, "explore", Action{})
		}
		claimed := map[int]bool{}
		for _, q := range g.Players {
			if q != p {
				claimed[q.Pos] = true
			}
		}
		open := func(i int) bool { return i > 0 && !g.Tiles[i].Up && !claimed[i] }
		likely := func(i int) bool { return open(i) && Board[i].Ring <= 2 }
		if b.any(likely) {
			return b.goToward(p, likely)
		}
		return b.goToward(p, open)
	}
	// 7. Everything done for me: gather on the World Tree.
	if p.Pos != 0 {
		return b.goToward(p, is(0))
	}
	return false
}
