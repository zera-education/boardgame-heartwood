package main

import "testing"

// Plays a whole 4-player game through the rules engine.
func TestFullGame(t *testing.T) {
	g := NewGame("TEST", "host")
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	host := func(typ string, a Action) error { a.Host, a.Type = "host", typ; return g.Apply(a) }
	as := func(p *Player, typ string, a Action) error {
		a.Pid, a.Secret, a.Type = p.ID, p.Secret, typ
		return g.Apply(a)
	}

	var ps []*Player
	for i, n := range []string{"Ann", "Ben", "Cat", "Dan"} {
		p, err := g.Join(n, Colors[i])
		must(err)
		ps = append(ps, p)
	}
	if _, err := g.Join("Ann", Colors[5]); err == nil {
		t.Fatal("duplicate name accepted")
	}
	// The Keeper assigns 1 to 3 powers to each player before the start.
	if host("start", Action{}) == nil {
		t.Fatal("started before powers were assigned")
	}
	if host("assignPowers", Action{Target: ps[0].ID, Types: []int{1, 2, 3, 4}}) == nil {
		t.Fatal("accepted 4 powers")
	}
	if host("assignPowers", Action{Target: ps[0].ID, Types: []int{2, 2}}) == nil {
		t.Fatal("accepted a repeated power")
	}
	for i, p := range ps {
		must(host("assignPowers", Action{Target: p.ID, Types: []int{i + 1, i + 5, 9}[:i%3+1]}))
	}
	must(host("start", Action{}))
	if host("assignPowers", Action{Target: ps[0].ID, Types: []int{3}}) == nil {
		t.Fatal("powers changed after the start")
	}

	// Plant everyone on the outer ring, in two neighbouring pairs.
	outer := []int{}
	for i, h := range g.Hexes {
		if h.Ring == 3 {
			outer = append(outer, i)
		}
	}
	for k := range ps {
		must(as(g.current(), "plant", Action{Hex: outer[k]}))
	}
	if g.Phase != PhaseTurn || g.Season != 1 {
		t.Fatalf("phase %s season %d", g.Phase, g.Season)
	}

	inward := func(p *Player) int {
		for i, h := range g.Hexes {
			if g.dist(p.Pos, i) == 1 && h.Ring == g.Hexes[p.Pos].Ring-1 {
				return i
			}
		}
		return -1
	}
	playSeason := func() {
		for n := 0; n < len(ps); n++ {
			p := g.current()
			if g.Season > 1 && g.Hexes[p.Pos].Ring > 1 {
				if to := inward(p); to >= 0 {
					p.Water += 2 // make sure the test can always afford the Oak
					if err := as(p, "step", Action{Hex: to}); err != nil {
						t.Logf("%s could not step in: %v", p.Name, err)
					}
				}
			}
			must(as(p, "endMove", Action{}))
			if g.Turn.Teleport {
				must(as(p, "endMove", Action{}))
			}
			if len(g.Turn.Choices) > 0 {
				must(as(p, "pickChoice", Action{Card: g.Turn.Choices[0]}))
			}
			must(as(p, "startShare", Action{}))
			must(as(p, "doneShare", Action{}))
			for _, o := range ps {
				if o != p {
					must(as(o, "trust", Action{}))
				}
			}
			if err := as(ps[0], "trust", Action{}); err == nil && p != ps[0] {
				t.Fatal("double trust accepted")
			}
			must(as(p, "endTrust", Action{}))
		}
		if g.Phase != PhaseDusk {
			t.Fatalf("expected dusk, got %s", g.Phase)
		}
		// Pair 0-1 and 2-3 if new partners, otherwise let the Keeper pair them.
		for i, p := range ps {
			o := ps[i^1]
			if g.Season > 1 {
				o = ps[(i+2)%4]
			}
			if contains(p.Partners, o.ID) {
				continue
			}
			must(as(p, "duskChoice", Action{Target: o.ID, Ally: true}))
		}
		must(host("resolveDusk", Action{}))
		if len(g.Dusk.Pairs) != 2 {
			t.Fatalf("pairs %+v", g.Dusk.Pairs)
		}
		must(host("nextSeason", Action{}))
	}
	playSeason()
	if len(ps[0].Allies) != 1 {
		t.Fatalf("expected Ann and Ben allied (adjacent), allies=%v", ps[0].Allies)
	}
	playSeason()
	playSeason()
	if g.Phase != PhaseGuess {
		t.Fatalf("expected guess, got %s", g.Phase)
	}
	for _, p := range ps {
		for _, o := range ps {
			if o != p {
				must(as(p, "guess", Action{Target: o.ID}))
				break
			}
		}
	}
	must(host("startChain", Action{}))
	for g.Phase == PhaseChain {
		giver := g.player(g.Chain[g.ChainIdx])
		must(as(g.player(giver.Target), "confirm", Action{N: 3}))
	}
	scores, _ := g.Scores()
	for _, s := range scores {
		t.Logf("%-4s trust %2d growth %2d alliances %2d secret %d total %2d winner=%v", s.Name, s.Trust, s.Growth, s.Alliances, s.Secret, s.Total, s.Winner)
	}
	if !scores[0].Winner || scores[0].Total == 0 {
		t.Fatal("no winner")
	}
	for _, p := range ps {
		if len(p.Cards) != 3 {
			t.Fatalf("%s kept %d cards", p.Name, len(p.Cards))
		}
	}
}

func TestBoardShape(t *testing.T) {
	hs := buildHexes()
	if len(hs) != 37 {
		t.Fatal(len(hs))
	}
	count := map[[2]int]int{}
	for _, h := range hs {
		count[[2]int{h.Ring, h.Region}]++
	}
	for r := 0; r < 6; r++ {
		for ring := 1; ring <= 3; ring++ {
			if count[[2]int{ring, r}] != ring {
				t.Fatalf("region %d ring %d has %d", r, ring, count[[2]int{ring, r}])
			}
		}
	}
	// Each region's spaces must touch: the inner space neighbours a middle one.
	g := &Game{Hexes: hs}
	for i, h := range hs {
		if h.Ring == 1 {
			ok := false
			for j, o := range hs {
				if o.Ring == 2 && o.Region == h.Region && g.dist(i, j) == 1 {
					ok = true
				}
			}
			if !ok {
				t.Fatalf("region %d is not a wedge", h.Region)
			}
		}
	}
}

// Rule fixes found in the playtest.
func TestPlaytestFixes(t *testing.T) {
	g := NewGame("FIX", "host")
	var ps []*Player
	for i, n := range []string{"A", "B", "C"} {
		p, _ := g.Join(n, Colors[i])
		g.Apply(Action{Host: "host", Type: "assignPowers", Target: p.ID, Types: []int{5, 1, 2}})
		ps = append(ps, p)
	}
	as := func(p *Player, typ string, a Action) error {
		a.Pid, a.Secret, a.Type = p.ID, p.Secret, typ
		return g.Apply(a)
	}
	g.Apply(Action{Host: "host", Type: "start"})
	outer := []int{}
	for i, h := range g.Hexes {
		if h.Ring == 3 {
			outer = append(outer, i)
		}
	}
	for k := range ps {
		as(g.current(), "plant", Action{Hex: outer[k*6]})
	}
	p := g.current()
	// Field Notes scouts only open rings: in Season 1, only Seedlands spaces.
	if err := as(p, "power", Action{Power: 5}); err != nil {
		t.Fatal(err)
	}
	for hex := range p.Peek {
		if g.Hexes[hex].Ring != 3 {
			t.Fatalf("scouted a closed ring: %d", g.Hexes[hex].Ring)
		}
	}
	// Stepping away cancels the claim.
	for to := range g.Hexes {
		if g.dist(p.Pos, to) == 1 && g.Hexes[to].Ring == 3 {
			if err := as(p, "step", Action{Hex: to}); err != nil {
				t.Fatal(err)
			}
			break
		}
	}
	if g.Turn.CanClaim {
		t.Fatal("could still claim after stepping away")
	}
	// A player can pass, and the next player's turn begins.
	if err := as(p, "pass", Action{}); err != nil {
		t.Fatal(err)
	}
	if g.current() == p || len(p.Cards) != 0 {
		t.Fatal("pass did not end the turn cleanly")
	}
	// Odd players: the third member of a pair still gets fireside water.
	for g.Phase == PhaseTurn {
		as(g.current(), "pass", Action{})
	}
	before := map[string]int{}
	for _, x := range ps {
		before[x.ID] = x.Water
	}
	g.Apply(Action{Host: "host", Type: "resolveDusk"})
	for _, pr := range g.Dusk.Pairs {
		if pr.C != "" && g.player(pr.C).Water != before[pr.C]+1 {
			t.Fatal("third member got no fireside water")
		}
	}
	// A full tie shares the win.
	g2 := &Game{Players: ps, ForestGoal: false}
	for _, x := range ps {
		x.Pot, x.Cards, x.Allies, x.Bonus, x.Confirmed, x.Guess = map[string]int{}, nil, nil, 0, 0, ""
		x.Target = ""
	}
	scores, _ := g2.Scores()
	winners := 0
	for _, s := range scores {
		if s.Winner {
			winners++
		}
	}
	if winners != 3 {
		t.Fatalf("expected a three-way shared win, got %d winners", winners)
	}
}
