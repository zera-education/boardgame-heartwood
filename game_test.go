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
	// Some spaces are empty clearings; make sure there's something next door to scout.
	for i := range g.Hexes {
		if i > 0 && g.dist(p.Pos, i) == 1 && g.Tokens[i] == nil {
			g.Tokens[i] = &Token{Kind: "mushroom"}
		}
	}
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
	// Odd players: everyone still gets a Dusk partner (one group is a trio).
	for g.Phase == PhaseTurn {
		as(g.current(), "pass", Action{})
	}
	g.Apply(Action{Host: "host", Type: "resolveDusk"})
	inPair := map[string]bool{}
	for _, pr := range g.Dusk.Pairs {
		inPair[pr.A], inPair[pr.B], inPair[pr.C] = true, true, true
	}
	for _, x := range ps {
		if !inPair[x.ID] {
			t.Fatalf("%s has no Dusk partner", x.Name)
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

// The Keeper taps public moves for a player; private ones stay on the phone.
func TestKeeperActsForPlayers(t *testing.T) {
	g := NewGame("KPR", "host")
	var ps []*Player
	for i, n := range []string{"A", "B"} {
		p, _ := g.Join(n, Colors[i])
		g.Apply(Action{Host: "host", Type: "assignPowers", Target: p.ID, Types: []int{3}})
		ps = append(ps, p)
	}
	keeper := func(as *Player, typ string, a Action) error {
		a.Host, a.As, a.Type = "host", as.ID, typ
		return g.Apply(a)
	}
	if err := g.Apply(Action{Host: "host", Type: "start"}); err != nil {
		t.Fatal(err)
	}
	outer := []int{}
	for i, h := range g.Hexes {
		if h.Ring == 3 {
			outer = append(outer, i)
		}
	}
	if keeper(ps[1], "plant", Action{Hex: outer[0]}) == nil {
		t.Fatal("Keeper planted for the wrong player")
	}
	for k, p := range ps {
		if err := keeper(p, "plant", Action{Hex: outer[k]}); err != nil {
			t.Fatalf("plant for %s: %v", p.Name, err)
		}
	}
	a := ps[0]
	for _, step := range []string{"power", "endMove", "startShare", "doneShare"} {
		if step == "startShare" && g.Turn.Teleport {
			keeper(a, "endMove", Action{}) // a Hidden path: stay here
		}
		if err := keeper(a, step, Action{Power: 3}); err != nil {
			t.Fatalf("%s: %v", step, err)
		}
	}
	if !a.Used[3] {
		t.Fatal("Momentum not used")
	}
	if keeper(ps[1], "trust", Action{}) == nil {
		t.Fatal("Keeper gave trust for a player")
	}
	if err := g.Apply(Action{Pid: ps[1].ID, Secret: ps[1].Secret, Type: "trust"}); err != nil {
		t.Fatalf("player's own trust: %v", err)
	}
	if err := keeper(a, "endTrust", Action{}); err != nil {
		t.Fatal(err)
	}
	if g.current().ID != ps[1].ID {
		t.Fatal("turn did not pass")
	}
}

// Moving is up to 3 steps a turn (Momentum adds 3), Open Hands shares the card,
// and an alliance costs nothing.
func TestStepsOpenHandsAndFreeAlliance(t *testing.T) {
	g := NewGame("STP", "host")
	var ps []*Player
	for i, n := range []string{"A", "B"} {
		p, _ := g.Join(n, Colors[i])
		g.Apply(Action{Host: "host", Type: "assignPowers", Target: p.ID, Types: []int{2, 3}})
		ps = append(ps, p)
	}
	as := func(p *Player, typ string, a Action) error {
		a.Pid, a.Secret, a.Type = p.ID, p.Secret, typ
		return g.Apply(a)
	}
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(g.Apply(Action{Host: "host", Type: "start"}))
	outer := []int{}
	for i, h := range g.Hexes {
		if h.Ring == 3 {
			outer = append(outer, i)
		}
	}
	// Plant both on the same space, so they're neighbours for the alliance.
	must(as(g.current(), "plant", Action{Hex: outer[0]}))
	must(as(g.current(), "plant", Action{Hex: outer[0]}))
	a, b := g.current(), ps[0]
	if a == ps[0] {
		b = ps[1]
	}
	walk := func(n int) error {
		for k := 0; k < n; k++ {
			next := -1
			for i, h := range g.Hexes {
				if h.Ring == 3 && g.dist(a.Pos, i) == 1 {
					next = i
					break
				}
			}
			if err := as(a, "step", Action{Hex: next}); err != nil {
				return err
			}
		}
		return nil
	}
	must(walk(3))
	if walk(1) == nil {
		t.Fatal("a fourth step was allowed")
	}
	must(as(a, "power", Action{Power: 3}))
	must(walk(3))
	must(as(a, "endMove", Action{}))
	if g.Turn.Teleport {
		must(as(a, "endMove", Action{}))
	}
	must(as(a, "power", Action{Power: 2, Target: b.ID}))
	must(as(a, "startShare", Action{}))
	must(as(a, "doneShare", Action{}))
	must(as(a, "endTrust", Action{}))
	if len(a.Cards) != 1 || len(b.Cards) != 1 || a.Bonus < 1 || b.Bonus < 1 {
		t.Fatalf("Open Hands: cards %d/%d, bonus %d/%d", len(a.Cards), len(b.Cards), a.Bonus, b.Bonus)
	}
	// b stays put; then both ask for an alliance and get it, with no cost.
	must(as(b, "endMove", Action{}))
	if g.Turn != nil && g.Turn.Teleport {
		must(as(b, "endMove", Action{}))
	}
	must(as(b, "startShare", Action{}))
	must(as(b, "doneShare", Action{}))
	must(as(b, "endTrust", Action{}))
	a.Pos = b.Pos
	must(as(a, "duskChoice", Action{Target: b.ID, Ally: true}))
	must(as(b, "duskChoice", Action{Target: a.ID, Ally: true}))
	must(g.Apply(Action{Host: "host", Type: "resolveDusk"}))
	if !hasAlly(a, b.ID) {
		t.Fatalf("no alliance: %+v", g.Dusk.Pairs)
	}
}
