package main

// Balance playtest: thousands of 10-player games played by bots through the
// real rules engine (Apply). Run with:
//
//	HW_SIM=1 go test -run TestBalanceSim -v
//
// Bots plan like reasonable players: each season they pick a Dusk partner they
// haven't talked to (one who stands for a different value, when they plan an
// alliance), then end their move as deep as the season allows. They use their powers when useful and give
// trust more often to deeper shares.

import (
	"fmt"
	"math/rand"
	"os"
	"sort"
	"strings"
	"testing"
)

type simStats struct {
	games                                         int
	alliances, heartwood, advocated, noWinner     float64
	trustUnused                                   float64
	oakShares, tierSum, shares                    float64
	winsByPower                                   [10]float64
	holdsPower                                    [10]float64
	scoreSum, trustSum, growthSum, advSum, secSum float64
	margin, spread                                float64
	zeroAlliance                                  float64
	stuckTurns                                    float64
}

type botOpts struct {
	players     int
	coordinate  bool // Season 3: bots spread across values that have no Oak story yet
	partnerPlan bool // bots pick Dusk partners they can ally with (different values, room left)
}

func TestBalanceSim(t *testing.T) {
	if os.Getenv("HW_SIM") == "" {
		t.Skip("set HW_SIM=1 to run the balance simulation")
	}
	runs := []struct {
		name string
		o    botOpts
	}{
		{"10 players, planning partners", botOpts{players: 10, partnerPlan: true, coordinate: true}},
		{"10 players, no partner planning", botOpts{players: 10, partnerPlan: false, coordinate: true}},
		{"10 players, no region coordination", botOpts{players: 10, partnerPlan: true, coordinate: false}},
		{"6 players, planning partners", botOpts{players: 6, partnerPlan: true, coordinate: true}},
	}
	for _, run := range runs {
		var st simStats
		for i := 0; i < 2000; i++ {
			simGame(t, run.o, &st)
		}
		report(t, run.name, &st, run.o)
	}
}

func report(t *testing.T, name string, s *simStats, o botOpts) {
	n := float64(s.games)
	pp := n * float64(o.players)
	var b strings.Builder
	fmt.Fprintf(&b, "\n=== %s (%d games)\n", name, s.games)
	fmt.Fprintf(&b, "allies per player: %.2f   players with no alliance: %.0f%%\n", s.alliances/pp, 100*s.zeroAlliance/pp)
	fmt.Fprintf(&b, "Heartwood shares per game: %.2f   Oak shares per game: %.2f   avg tier: %.2f\n", s.heartwood/n, s.oakShares/n, s.tierSum/s.shares)
	fmt.Fprintf(&b, "values advocated per game: %.1f of 6\n", s.advocated/n)
	fmt.Fprintf(&b, "end of game per player: trust acorns unused %.1f\n", s.trustUnused/pp)
	fmt.Fprintf(&b, "avg score %.1f = trust %.1f + growth %.1f + advocacy %.1f + secret %.1f\n", s.scoreSum/pp, s.trustSum/pp, s.growthSum/pp, s.advSum/pp, s.secSum/pp)
	fmt.Fprintf(&b, "winner margin %.1f points, top-to-bottom spread %.1f\n", s.margin/n, s.spread/n)
	fmt.Fprintf(&b, "turns where the bot could not move at all: %.1f%%\n", 100*s.stuckTurns/(pp*3))
	fmt.Fprintf(&b, "win rate when holding each power (fair = %.1f%%):\n", 100/float64(o.players))
	for ty := 1; ty <= 9; ty++ {
		fmt.Fprintf(&b, "  %d %-13s %-13s %.1f%%\n", ty, Powers[ty-1].Name, Powers[ty-1].Title, 100*s.winsByPower[ty]/s.holdsPower[ty])
	}
	t.Log(b.String())
}

func simGame(t *testing.T, o botOpts, st *simStats) {
	g := NewGame("SIM", "host")
	host := func(typ string, a Action) error { a.Host, a.Type = "host", typ; return g.Apply(a) }
	as := func(p *Player, typ string, a Action) error {
		a.Pid, a.Secret, a.Type = p.ID, p.Secret, typ
		return g.Apply(a)
	}
	must := func(err error, what string) {
		if err != nil {
			t.Fatalf("%s: %v", what, err)
		}
	}
	openness := map[string]float64{}
	for i := 0; i < o.players; i++ {
		types := rand.Perm(9)[:3]
		for k := range types {
			types[k]++
		}
		p, err := g.Join(fmt.Sprintf("P%d", i), Colors[i])
		must(err, "join")
		must(host("assignPowers", Action{Target: p.ID, Types: types}), "assign powers")
		openness[p.ID] = 0.7 + 0.6*rand.Float64() // some people's stories land harder than others'
	}
	must(host("start", Action{}), "start")

	// Plant: anywhere on the outer ring.
	for g.Phase == PhasePlant {
		outer := []int{}
		for i, h := range g.Hexes {
			if h.Ring == 3 {
				outer = append(outer, i)
			}
		}
		must(as(g.current(), "plant", Action{Hex: outer[rand.Intn(len(outer))]}), "plant")
	}

	for season := 1; season <= 3; season++ {
		// Each bot picks a Dusk partner for this season.
		buddy := pickBuddies(g, o.partnerPlan)
		regionTaken := map[int]bool{}
		for g.Phase == PhaseTurn {
			p := g.current()
			if !botTurn(g, p, o, buddy[p.ID], regionTaken, openness, as) {
				st.stuckTurns++
			}
		}
		if g.Phase != PhaseDusk {
			t.Fatalf("expected dusk, got %s", g.Phase)
		}
		// Dusk: choose the buddy; ask for an alliance when it can work.
		for _, p := range g.Players {
			b := g.player(buddy[p.ID])
			if b == nil {
				continue
			}
			ally := g.allyCheck(p, b, map[string]bool{}) == ""
			if hasPower(p, 9) && !p.Used[9] && season == 3 {
				as(p, "power", Action{Power: 9})
			}
			as(p, "duskChoice", Action{Target: b.ID, Ally: ally})
		}
		must(host("resolveDusk", Action{}), "resolve")
		for _, al := range g.Alliances {
			if al.Season == g.Season && al.Statement == "" {
				must(as(g.player(al.Members[0]), "statement", Action{Text: "We hold " + g.valueKey(al.Members) + " together."}), "statement")
			}
		}
		must(host("nextSeason", Action{}), "next season")
	}

	// Finale: alliance stories, then guesses.
	for g.Phase == PhaseStories {
		must(host("nextStory", Action{}), "story")
	}
	for _, p := range g.Players {
		guess := g.Players[rand.Intn(len(g.Players))]
		if rand.Float64() < 0.3 {
			guess = g.owlOf(p.ID)
		}
		if guess.ID != p.ID {
			as(p, "guess", Action{Target: guess.ID})
		}
	}
	must(host("startChain", Action{}), "chain")
	for g.Phase == PhaseChain {
		giver := g.player(g.Chain[g.ChainIdx])
		must(as(g.player(giver.Target), "confirm", Action{N: 2 + rand.Intn(2)}), "confirm")
	}

	scores := g.Scores()
	st.games++
	for v := range Values {
		if g.advocated(v) {
			st.advocated++
		}
	}
	if !scores[0].Winner {
		st.noWinner++
	}
	st.margin += float64(scores[0].Total - scores[1].Total)
	st.spread += float64(scores[0].Total - scores[len(scores)-1].Total)
	for _, s := range scores {
		p := g.player(s.ID)
		st.scoreSum += float64(s.Total)
		st.trustSum += float64(s.Trust)
		st.growthSum += float64(s.Growth)
		st.advSum += float64(s.Advocacy)
		st.secSum += float64(s.Secret)
		st.alliances += float64(len(p.Allies))
		if len(p.Allies) == 0 {
			st.zeroAlliance++
		}
		st.trustUnused += float64(p.TrustLeft)
		for _, c := range p.Cards {
			st.shares++
			st.tierSum += float64(c.Tier)
			if c.Tier == 3 {
				st.oakShares++
			}
			if c.Tier == 4 {
				st.heartwood++
			}
		}
		for _, ty := range p.Types {
			st.holdsPower[ty]++
			if s.ID == scores[0].ID {
				st.winsByPower[ty]++
			}
		}
	}
}

func hasPower(p *Player, ty int) bool {
	for _, x := range p.Types {
		if x == ty {
			return true
		}
	}
	return false
}

// pickBuddies pairs players who haven't talked yet, preferring people close by,
// and (when planning) people they could ally with.
func pickBuddies(g *Game, plan bool) map[string]string {
	ids := append([]string(nil), g.Order...)
	rand.Shuffle(len(ids), func(i, j int) { ids[i], ids[j] = ids[j], ids[i] })
	out := map[string]string{}
	for _, id := range ids {
		if out[id] != "" {
			continue
		}
		p := g.player(id)
		best, bestD := "", 99
		for _, o := range ids {
			if o == id || out[o] != "" || contains(p.Partners, o) {
				continue
			}
			d := g.dist(p.Pos, g.player(o).Pos) + rand.Intn(3)
			if plan && g.allyCheck(p, g.player(o), map[string]bool{}) == "" {
				d -= 10
			}
			if d < bestD {
				best, bestD = o, d
			}
		}
		if best != "" {
			out[id], out[best] = best, id
		}
	}
	return out
}

type reach struct {
	hex  int
	path []int
}

// reachable lists every space the player can end on this turn within the steps
// left, with the shortest path to it.
func reachable(g *Game, p *Player, steps int) []reach {
	start := reach{hex: p.Pos}
	seen := map[int]bool{p.Pos: true}
	out := []reach{start}
	queue := []reach{start}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if len(cur.path) >= steps {
			continue
		}
		for to := range g.Hexes {
			if seen[to] || g.dist(cur.hex, to) != 1 || g.canEnter(p, to) != nil {
				continue
			}
			seen[to] = true
			nx := reach{to, append(append([]int(nil), cur.path...), to)}
			out = append(out, nx)
			queue = append(queue, nx)
		}
	}
	return out
}

func botTurn(g *Game, p *Player, o botOpts, buddy string, regionTaken map[int]bool, openness map[string]float64,
	as func(*Player, string, Action) error) bool {
	tu := g.Turn
	// Powers that help movement.
	if hasPower(p, 3) && !p.Used[3] && g.Season >= 2 {
		as(p, "power", Action{Power: 3})
	}
	if hasPower(p, 7) && !p.Used[7] {
		as(p, "power", Action{Power: 7}) // a Campfire round, any time
	}
	if hasPower(p, 5) && !p.Used[5] && g.Season == 1 && as(p, "power", Action{Power: 5}) == nil {
		for hex := range p.Peek {
			as(p, "claim", Action{Hex: hex})
			break
		}
	}
	if hasPower(p, 6) && !p.Used[6] && g.Season == 3 {
		for _, a := range p.Allies {
			if ap := g.player(a); ap != nil && g.Hexes[ap.Pos].Ring <= 1 {
				as(p, "power", Action{Power: 6, Target: a})
				break
			}
		}
	}

	// Choose where to end: deep as the season allows, near the buddy, a hidden discovery, and
	// in Season 3 a region without an Oak story yet (if bots coordinate).
	options := reachable(g, p, tu.StepsLeft)
	b := g.player(buddy)
	bestScore, best := -1e9, options[0]
	for _, r := range options {
		h := g.Hexes[r.hex]
		s := float64(3-h.Ring) * 3 // deeper is worth more
		if h.Ring == 0 {
			s = 10
		}
		if o.partnerPlan && b != nil && g.dist(r.hex, b.Pos) <= 1 {
			s += 4
		}
		if t := g.Tokens[r.hex]; t != nil && !t.Flipped {
			s += 1
		}
		if g.Season == 3 && o.coordinate && h.Ring == 1 && !g.Oak[h.Region] && !regionTaken[h.Region] {
			s += 5
		}
		s += rand.Float64()
		if s > bestScore {
			bestScore, best = s, r
		}
	}
	moved := len(options) > 1
	for _, hex := range best.path {
		if err := as(p, "step", Action{Hex: hex}); err != nil {
			break
		}
	}
	as(p, "endMove", Action{})
	if g.Turn.Teleport {
		as(p, "endMove", Action{})
	}
	if hasPower(p, 1) && !p.Used[1] && g.Turn.Tier < 4 && as(p, "power", Action{Power: 1}) == nil {
		as(p, "pickChoice", Action{Card: g.Turn.Choices[0]})
	}
	if hasPower(p, 2) && !p.Used[2] && g.Season >= 2 {
		// Open Hands: invite someone to answer the question too.
		for _, x := range g.Players {
			if x.ID != p.ID && rand.Float64() < 0.3 && as(p, "power", Action{Power: 2, Target: x.ID}) == nil {
				break
			}
		}
	}
	if hasPower(p, 4) && !p.Used[4] && g.Turn.Tier < 3 && g.Season >= 2 {
		as(p, "power", Action{Power: 4})
	}
	if g.Turn.Tier == 3 {
		regionTaken[g.Turn.Region] = true
	}
	as(p, "startShare", Action{})
	as(p, "doneShare", Action{})
	// Listeners: deeper shares and some people's stories draw more trust.
	rate := []float64{0, 0.25, 0.4, 0.55, 0.65}[g.Turn.Tier] * openness[p.ID]
	listeners := append([]*Player(nil), g.Players...)
	sort.Slice(listeners, func(i, j int) bool { return listeners[i].ID < listeners[j].ID })
	for _, x := range listeners {
		if x.ID == p.ID {
			continue
		}
		if hasPower(x, 8) && !x.Used[8] && g.Season == 3 && rand.Float64() < 0.3 {
			if as(x, "power", Action{Power: 8}) == nil {
				continue
			}
		}
		if rand.Float64() < rate {
			as(x, "trust", Action{})
		}
		if x.Squirrels > 0 && rand.Float64() < 0.2 {
			as(x, "squirrel", Action{})
		}
	}
	as(p, "endTrust", Action{})
	return moved
}
