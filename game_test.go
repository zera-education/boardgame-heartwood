package main

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// table is a test game with its players, played through Apply the way the
// big screen and phones do: the Keeper acts "as" players, phones send trust,
// the Investigator's look and the Secret Owl guess.
type table struct {
	t  *testing.T
	g  *Game
	ps []*Player
}

// newTable joins n players and gives each the types from types(i) (default:
// one type each, 1..9 in turn).
func newTable(t *testing.T, n int, types func(i int) []int) *table {
	t.Helper()
	x := &table{t: t, g: NewGame("TEST", "host")}
	for i := 0; i < n; i++ {
		p, err := x.g.Join(fmt.Sprintf("P%d", i), Colors[i])
		x.must(err)
		x.ps = append(x.ps, p)
	}
	for i, p := range x.ps {
		ty := []int{i%9 + 1}
		if types != nil {
			ty = types(i)
		}
		x.must(x.host("assignPowers", Action{Target: p.ID, Types: ty}))
	}
	return x
}

func (x *table) must(err error) {
	x.t.Helper()
	if err != nil {
		x.t.Fatal(err)
	}
}

func (x *table) fails(err error, what string) {
	x.t.Helper()
	if err == nil {
		x.t.Fatalf("%s: accepted", what)
	}
}

func (x *table) host(typ string, a Action) error {
	a.Host, a.Type = "host", typ
	return x.g.Apply(a)
}

func (x *table) as(p *Player, typ string, a Action) error {
	a.Host, a.As, a.Type = "host", p.ID, typ
	return x.g.Apply(a)
}

func (x *table) phone(p *Player, typ string, a Action) error {
	a.Pid, a.Secret, a.Type = p.ID, p.Secret, typ
	return x.g.Apply(a)
}

// own is an action on the player's own hex (the HTTP API defaults hex to -1).
var own = Action{Hex: -1}

// play starts the game and enters everyone on the outer ring of value i%6,
// closing each "why" share. Then it calms the forest for predictable tests:
// sun now (no effect on actions), fog at the next Tide (no effect), no Breath,
// every tile face down and empty.
func (x *table) play() {
	x.t.Helper()
	x.must(x.host("start", Action{}))
	for i, p := range x.ps {
		x.must(x.as(p, "enter", Action{N: i % 6, Hex: ringHexes(4, i%6)[0]}))
		x.must(x.host("doneShare", Action{}))
	}
	x.calm()
	for i := 1; i < len(Board); i++ {
		x.g.Tiles[i] = &Tile{Kind: "empty"}
	}
}

// play2 enters everyone in a game that has already started.
func (x *table) play2() {
	x.t.Helper()
	for i, p := range x.ps {
		x.must(x.as(p, "enter", Action{N: i % 6, Hex: ringHexes(4, i%6)[0]}))
		x.must(x.host("doneShare", Action{}))
	}
	x.calm()
}

func (x *table) calm() {
	for s := range 6 {
		x.g.Weather[s], x.g.NextWeather[s] = "sun", "fog"
	}
	x.g.Breath = nil
}

// turnOf makes it p's turn with fresh actions.
func (x *table) turnOf(p *Player) {
	x.g.TurnIdx = slices.Index(x.g.Order, p.ID)
	x.g.newTurn()
}

// endRound ends every remaining turn of the round, which runs the Forest Tide.
func (x *table) endRound() {
	x.t.Helper()
	for x.g.Phase == PhaseTurn {
		last := x.g.TurnIdx == len(x.g.Order)-1
		x.must(x.host("endTurn", Action{}))
		if last {
			return
		}
	}
}

func (x *table) lastEvent(typ string) Event {
	for i := len(x.g.Events) - 1; i >= 0; i-- {
		if x.g.Events[i]["type"] == typ {
			return x.g.Events[i]
		}
	}
	return nil
}

func ringHexes(ring, sector int) []int {
	var out []int
	for i, h := range Board {
		if h.Ring == ring && (sector < 0 || h.Sector == sector) {
			out = append(out, i)
		}
	}
	return out
}

func inwardOf(i int) int {
	for _, j := range Adj[i] {
		if Board[j].Ring == Board[i].Ring-1 {
			return j
		}
	}
	return -1
}

func TestBoardShape(t *testing.T) {
	if len(Board) != 61 {
		t.Fatalf("%d hexes", len(Board))
	}
	if Board[0].Ring != 0 || Board[0].Sector != -1 {
		t.Fatal("hex 0 is not the World Tree")
	}
	per := map[int]int{}
	for _, h := range Board[1:] {
		per[h.Sector]++
	}
	for s := range 6 {
		if per[s] != 10 {
			t.Fatalf("sector %d has %d hexes", s, per[s])
		}
	}
	// The top of the board (the first ring hex walks from direction 4) reads Z first.
	if len(Adj[0]) != 6 || len(ringHexes(1, -1)) != 6 || len(ringHexes(4, -1)) != 24 {
		t.Fatal("rings are the wrong size")
	}
	for k := 0; k < 50; k++ {
		g := NewGame("T", "h")
		springs, treasures := 0, map[string]bool{}
		for i, tl := range g.Tiles {
			if i == 0 {
				if tl != nil {
					t.Fatal("the World Tree has a tile")
				}
				continue
			}
			if tl.Up {
				t.Fatal("a tile starts face up")
			}
			switch tl.Kind {
			case "spring":
				springs++
			case "treasure":
				if Board[i].Ring > 2 {
					t.Fatalf("treasure in ring %d", Board[i].Ring)
				}
				treasures[tl.Treasure] = true
			}
		}
		if springs != 4 || len(treasures) != 3 {
			t.Fatalf("springs %d treasures %v", springs, treasures)
		}
		if g.Rules != RulesVersion || len(g.Breath) != 2 {
			t.Fatalf("rules %q breath %d", g.Rules, len(g.Breath))
		}
		for _, d := range g.Breath {
			if d.To == 0 || (Board[d.From].Ring > 1 && Board[d.To].Ring != Board[d.From].Ring-1) || (Board[d.From].Ring == 1 && d.To != d.From) {
				t.Fatalf("bad drift %+v", d)
			}
		}
	}
}

func TestPlayerLimits(t *testing.T) {
	if len(Colors) != 12 {
		t.Fatalf("%d colours", len(Colors))
	}
	x := newTable(t, 5, nil)
	x.fails(x.host("start", Action{}), "start with 5 players")
	p, err := x.g.Join("P5", Colors[5])
	x.must(err)
	x.ps = append(x.ps, p)
	x.fails(x.host("start", Action{}), "start with a player without a type")
	x.fails(x.host("assignPowers", Action{Target: p.ID, Types: []int{1, 2, 3, 4}}), "4 types")
	x.fails(x.host("assignPowers", Action{Target: p.ID, Types: []int{2, 2}}), "a repeated type")
	x.fails(x.host("assignPowers", Action{Target: p.ID, Types: []int{10}}), "type 10")
	x.must(x.host("assignPowers", Action{Target: p.ID, Types: []int{5, 9, 1}}))
	x.fails(x.phone(p, "assignPowers", Action{Target: p.ID, Types: []int{1}}), "a player assigning types")
	if _, err := x.g.Join("P0", Colors[7]); err == nil {
		t.Fatal("duplicate name accepted")
	}
	if _, err := x.g.Join("Zed", Colors[0]); err == nil {
		t.Fatal("duplicate colour accepted")
	}
	for i := 6; i < 12; i++ {
		q, err := x.g.Join(fmt.Sprintf("P%d", i), Colors[i])
		x.must(err)
		x.must(x.host("assignPowers", Action{Target: q.ID, Types: []int{i%9 + 1}}))
	}
	if _, err := x.g.Join("P12", "#000000"); err == nil {
		t.Fatal("a 13th player joined")
	}
	// Kick works in the lobby only.
	x.must(x.host("kick", Action{Target: x.g.Players[11].ID}))
	if len(x.g.Players) != 11 {
		t.Fatal("kick failed")
	}
	x.must(x.host("start", Action{}))
	if x.g.Phase != PhaseEnter || len(x.g.Order) != 11 {
		t.Fatalf("phase %s order %d", x.g.Phase, len(x.g.Order))
	}
	x.fails(x.host("kick", Action{Target: p.ID}), "kick after the start")
	x.fails(x.host("assignPowers", Action{Target: p.ID, Types: []int{3}}), "types changed after the start")
	if _, err := x.g.Join("Late", "#000000"); err == nil {
		t.Fatal("joined after the start")
	}
	// Secret Owl: one loop through everyone.
	seen := map[string]bool{}
	for _, q := range x.g.Players {
		if q.Target == "" || q.Target == q.ID || seen[q.Target] || q.TrustLeft != 10 || q.Water != 4 {
			t.Fatalf("bad deal for %s: %+v", q.Name, q)
		}
		seen[q.Target] = true
	}
}

// Plays a whole 6-player game: enter with why shares, turns and a Forest Tide,
// a forced win, then the finale with recognition.
func TestFullGame(t *testing.T) {
	x := newTable(t, 6, nil)
	ps, g := x.ps, x.g
	x.must(x.host("start", Action{}))

	// Entering: in join order, on the outer ring of your value, then say why.
	x.fails(x.as(ps[1], "enter", Action{N: 1, Hex: ringHexes(4, 1)[0]}), "entering out of turn")
	x.fails(x.as(ps[0], "enter", Action{N: 0, Hex: ringHexes(3, 0)[0]}), "entering on ring 3")
	x.fails(x.as(ps[0], "enter", Action{N: 0, Hex: ringHexes(4, 1)[0]}), "entering in another value's sector")
	x.fails(x.as(ps[0], "move", Action{Hex: 1}), "moving during enter")
	for i, p := range ps {
		v := i % 6
		x.must(x.as(p, "enter", Action{N: v, Hex: ringHexes(4, v)[1]}))
		s := g.openShare()
		if s == nil || s.Kind != "why" || s.Player != p.ID || !strings.Contains(s.Prompt, Values[v].Name) || s.Sub != Values[v].Tagline {
			t.Fatalf("why share %+v", s)
		}
		if i+1 < len(ps) {
			x.fails(x.as(ps[i+1], "enter", Action{N: (i + 1) % 6, Hex: ringHexes(4, (i+1)%6)[1]}), "entering while a share is open")
		}
		x.fails(x.phone(p, "trust", Action{}), "trusting your own share")
		x.fails(x.as(ps[(i+1)%6], "trust", Action{}), "the Keeper giving trust")
		listener := ps[(i+1)%6]
		x.must(x.phone(listener, "trust", Action{}))
		x.fails(x.phone(listener, "trust", Action{}), "two acorns for one share")
		x.must(x.host("doneShare", Action{}))
	}
	x.fails(x.host("doneShare", Action{}), "doneShare with no share")
	if g.Phase != PhaseTurn || g.Round != 1 || g.Turn.Player != ps[0].ID || g.Turn.Actions != 2 {
		t.Fatalf("phase %s round %d turn %+v", g.Phase, g.Round, g.Turn)
	}
	for _, p := range ps {
		if p.TrustLeft != 9 || p.Pot[ps[(slices.Index(ps, p)+1)%6].ID] != 1 {
			t.Fatalf("trust after the why round: %s left %d pot %v", p.Name, p.TrustLeft, p.Pot)
		}
	}

	// A round: everyone ends their turn, then the Forest Tide runs by itself.
	x.calm()
	x.endRound()
	if g.Round != 2 || g.Tide != 1 || g.TurnIdx != 0 || g.Weather != [6]string{"fog", "fog", "fog", "fog", "fog", "fog"} {
		t.Fatalf("after the first round: round %d tide %d weather %v", g.Round, g.Tide, g.Weather)
	}
	if x.lastEvent("tide") == nil || len(g.Breath) != 2 {
		t.Fatalf("no tide event, or breath %d", len(g.Breath))
	}
	x.calm()
	x.endRound()
	if g.Tide != 2 || len(g.Breath) != 3 {
		t.Fatalf("tide %d breath %d (want 3 after 2 tides)", g.Tide, len(g.Breath))
	}

	// Force a near-win: a Big Tree in every value, two treasures placed, five
	// players on the World Tree having placed fruit. The last one walks in with
	// a fruit and the Rope.
	for s := range 6 {
		g.Tiles[ringHexes(3, s)[0]] = &Tile{Up: true, Kind: "empty", Stage: 4}
	}
	g.Placed = []string{"compass", "lantern"}
	for _, p := range ps[:5] {
		p.Pos, p.Placed = 0, 1
	}
	last := ps[5]
	last.Pos, last.Fruit, last.Treasure = ringHexes(1, -1)[0], 1, "rope"
	x.turnOf(last)
	x.must(x.as(last, "endTurn", Action{})) // nothing happens until the tree check
	x.turnOf(last)
	if g.Phase != PhaseTurn {
		t.Fatal("won too early")
	}
	x.must(x.as(last, "move", Action{Hex: 0}))
	if g.Result != "won" || g.Phase != PhaseGuess || last.Placed != 1 || len(g.Placed) != 3 || g.EndedAt == 0 {
		t.Fatalf("result %q phase %s placed %d treasures %v ended at %d", g.Result, g.Phase, last.Placed, g.Placed, g.EndedAt)
	}
	if x.lastEvent("wake") == nil || x.lastEvent("place") == nil {
		t.Fatal("missing wake or place event")
	}
	logs := strings.Join(g.Log, "\n")
	if !strings.Contains(logs, "The forest wakes!") || !strings.Contains(logs, HeartComplete) {
		t.Fatalf("log: %s", logs)
	}
	if s := g.openShare(); s == nil || s.Kind != "heartwood" || s.Player != last.ID {
		t.Fatalf("expected the last fruit's Heartwood question, got %+v", s)
	}
	x.must(x.phone(ps[1], "trust", Action{})) // trust still works during the finale's open share
	x.fails(x.as(ps[0], "move", Action{Hex: 1}), "moving after the game ended")

	// Finale: guess your Secret Owl (even players right, odd ones wrong).
	x.fails(x.phone(ps[0], "guess", Action{Target: ps[0].ID}), "guessing yourself")
	x.fails(x.as(ps[0], "guess", Action{Target: ps[1].ID}), "the Keeper guessing")
	for i, p := range ps {
		owl := g.owlOf(p.ID)
		guess := owl.ID
		if i%2 == 1 {
			for _, q := range ps {
				if q != p && q != owl {
					guess = q.ID
					break
				}
			}
		}
		x.must(x.phone(p, "guess", Action{Target: guess}))
	}
	x.fails(x.host("startChain", Action{}), "the chain while a share is open")
	x.must(x.host("doneShare", Action{}))
	x.must(x.host("startChain", Action{}))
	v := buildView(g, "", "", "host")
	tr, ok := v["tribute"].(map[string]any)
	if !ok || tr["total"] != 6 || tr["giver"] != g.Chain[0] {
		t.Fatalf("tribute %v", v["tribute"])
	}
	for range 6 {
		x.must(x.host("nextTribute", Action{}))
	}
	if g.Phase != PhaseEnd {
		t.Fatalf("phase %s", g.Phase)
	}
	rec := g.Recognition()
	for i, r := range rec {
		p := ps[i]
		wantTrust := 1
		if p == last {
			wantTrust = 2 // the why share and the Heartwood question
		}
		if r.Trust != wantTrust || r.OwlName != g.owlOf(p.ID).Name || r.GuessedRight != (i%2 == 0) {
			t.Fatalf("recognition %+v", r)
		}
		target := g.player(p.Target)
		if r.OwlHidden != (target.Guess != p.ID) || r.TargetName != target.Name {
			t.Fatalf("owl hidden %+v", r)
		}
	}
	if rec[5].Givers != 2 {
		t.Fatalf("givers %d", rec[5].Givers)
	}
	if buildView(g, "", "", "host")["recognition"] == nil {
		t.Fatal("no recognition in the end view")
	}
}

// Every action and its errors, for plain players (Peacemakers only change passing).
func TestActions(t *testing.T) {
	x := newTable(t, 6, func(i int) []int { return []int{9} })
	x.play()
	g, p, q := x.g, x.ps[0], x.ps[1]
	h := ringHexes(3, 0)[0]
	p.Pos = h
	x.turnOf(p)

	x.fails(x.as(q, "explore", own), "acting out of turn")
	x.fails(x.as(p, "sow", own), "sowing a face-down hex")
	x.fails(x.as(p, "fly", own), "an unknown action")
	x.must(x.as(p, "explore", own))
	if !g.Tiles[h].Up || g.Turn.Actions != 1 || x.lastEvent("flip")["hex"] != h {
		t.Fatal("explore")
	}
	x.fails(x.as(p, "explore", own), "exploring twice")
	x.must(x.as(p, "sow", own))
	if g.Tiles[h].Stage != 1 || x.lastEvent("grow")["stage"] != 1 {
		t.Fatal("sow")
	}
	x.fails(x.as(p, "water", own), "a third action")
	x.turnOf(p)
	x.fails(x.as(p, "sow", own), "sowing where something grows")
	x.fails(x.as(p, "tend", own), "tending a seed")
	x.must(x.as(p, "water", own))
	x.must(x.as(p, "water", own))
	if g.Tiles[h].Stage != 3 || p.Water != 2 || g.Log[len(g.Log)-1] != "P0 waters it: Sapling." {
		t.Fatalf("water: stage %d water %d log %q", g.Tiles[h].Stage, p.Water, g.Log[len(g.Log)-1])
	}
	x.turnOf(p)
	x.fails(x.as(p, "water", own), "watering a Sapling")
	x.fails(x.as(p, "harvest", own), "harvesting a Sapling")
	x.must(x.as(p, "tend", own))
	if g.Tiles[h].Stage != 4 || !g.goals().Trees[0] {
		t.Fatal("tend")
	}
	x.must(x.as(p, "harvest", own))
	s := g.openShare()
	if p.Fruit != 1 || s == nil || s.Kind != "harvest" || s.Prompt != "Tell us a story from your own life about Zealous." || s.Sub != Values[0].Tagline {
		t.Fatalf("harvest: fruit %d share %+v", p.Fruit, s)
	}
	if e := x.lastEvent("harvest"); e["hex"] != h || e["pid"] != p.ID {
		t.Fatalf("harvest event %v", e)
	}
	x.turnOf(p)
	x.fails(x.as(p, "move", Action{Hex: Adj[h][0]}), "moving while a share is open")
	x.fails(x.as(p, "endTurn", own), "ending the turn while a share is open")
	x.must(x.host("doneShare", Action{}))
	x.fails(x.as(p, "harvest", own), "harvesting twice before the Tide")
	g.Tiles[h].Harvested = false
	g.Weather[0] = "rain"
	x.fails(x.as(p, "harvest", own), "harvesting in the rain")
	g.Weather[0] = "sun"
	p.Fruit = 2
	x.fails(x.as(p, "harvest", own), "a third fruit")
	g.Tiles[h].Leaves = 2
	p.Fruit = 0
	x.fails(x.as(p, "harvest", own), "harvesting a sealed tree")
	g.Tiles[h].Leaves = 0

	// Water: needs your own water.
	w := Adj[h][0]
	g.Tiles[w] = &Tile{Up: true, Kind: "empty", Stage: 1}
	p.Pos = w
	p.Water = 0
	x.turnOf(p)
	x.fails(x.as(p, "water", own), "watering with no water")
	x.fails(x.as(p, "move", Action{Hex: h}), "moving with no water")
	p.Water = 3
	x.fails(x.as(p, "water", Action{Hex: h}), "a non-Helper watering a neighbour")

	// Move: next door only, never into a sealed hex; you can walk out of one.
	far := ringHexes(4, 3)[0]
	x.fails(x.as(p, "move", Action{Hex: far}), "moving far")
	x.fails(x.as(p, "move", Action{Hex: p.Pos}), "moving in place")
	g.Tiles[h].Leaves = 2
	x.fails(x.as(p, "move", Action{Hex: h}), "moving into a sealed hex")
	g.Tiles[w].Leaves = 2 // standing on a hex that seals: still free to leave
	var out int
	for _, j := range Adj[w] {
		if j != h && j != 0 {
			out = j
			break
		}
	}
	x.must(x.as(p, "move", Action{Hex: out}))
	if p.Pos != out || x.lastEvent("move")["to"] != out {
		t.Fatal("move")
	}

	// Clear: your hex or a neighbour, 2 actions in rain.
	x.turnOf(p)
	x.fails(x.as(p, "clear", Action{Hex: far}), "clearing far away")
	x.fails(x.as(p, "clear", own), "clearing a hex with no leaves")
	g.Weather[Board[w].Sector] = "rain"
	x.must(x.as(p, "clear", Action{Hex: w}))
	if g.Tiles[w].Leaves != 1 || g.Turn.Actions != 0 {
		t.Fatalf("clear in rain: leaves %d actions %d", g.Tiles[w].Leaves, g.Turn.Actions)
	}
	g.Weather[Board[w].Sector] = "sun"
	x.turnOf(p)
	x.fails(x.as(p, "clear", Action{Hex: 0}), "clearing the World Tree")
}

func TestSpringsTreasuresAndFog(t *testing.T) {
	x := newTable(t, 6, func(i int) []int { return []int{9} })
	x.play()
	g, p, q := x.g, x.ps[0], x.ps[1]
	h := ringHexes(2, 1)[0]
	p.Pos = h

	// Explore in fog costs 2 actions.
	x.turnOf(p)
	g.Weather[1] = "fog"
	g.Turn.Actions = 1
	x.fails(x.as(p, "explore", own), "exploring fog with 1 action")
	x.turnOf(p)
	g.Tiles[h] = &Tile{Kind: "spring"}
	x.must(x.as(p, "explore", own))
	if g.Turn.Actions != 0 || !g.Tiles[h].Up {
		t.Fatal("explore in fog costs 2")
	}

	// Springs: drink back to 5; never run dry; nothing grows there.
	x.turnOf(p)
	x.fails(x.as(p, "sow", own), "sowing on a spring")
	p.Water = 5
	x.fails(x.as(p, "drink", own), "drinking when full")
	p.Water = 1
	x.must(x.as(p, "drink", own))
	p.Water = 0
	x.must(x.as(p, "drink", own))
	if p.Water != 5 || x.lastEvent("drink")["pid"] != p.ID {
		t.Fatal("drink")
	}
	x.turnOf(p)
	x.fails(x.as(p, "drink", own), "drinking when full again")

	// A treasure: revealing it is a group moment, then it can be taken.
	tr := ringHexes(1, 2)[0]
	g.Tiles[tr] = &Tile{Kind: "treasure", Treasure: "lantern"}
	p.Pos, q.Pos = tr, tr
	x.turnOf(p)
	x.fails(x.as(p, "take", own), "taking a hidden treasure")
	x.must(x.as(p, "explore", own))
	if e := x.lastEvent("treasure"); e["hex"] != tr || e["treasure"] != "lantern" {
		t.Fatalf("treasure event %v", e)
	}
	s := g.openShare()
	if s == nil || s.Kind != "treasure" || s.Player != p.ID || s.Sub != "lantern" || s.Prompt != treasureByID("lantern").Question {
		t.Fatalf("treasure share %+v", s)
	}
	v := buildView(g, q.ID, q.Secret, "")
	sv := v["share"].(map[string]any)
	if sv["sub"] != "lantern" || sv["kind"] != "treasure" {
		t.Fatalf("view share %v", sv)
	}
	x.fails(x.phone(q, "trust", Action{}), "trust for a treasure moment")
	x.fails(x.as(p, "take", own), "taking while the group answers")
	// everyone answers in turn, the finder first, each with their own 10-second timer
	if len(g.Shares) != len(g.Order) || s.By != p.ID || s.Seq != 1 || s.Of != len(g.Order) || sv["by"] != p.ID {
		t.Fatalf("treasure round: %d shares for %d players, first %+v", len(g.Shares), len(g.Order), s)
	}
	seen := map[string]bool{}
	for k := 1; k <= len(g.Order); k++ {
		s := g.openShare()
		if s == nil || s.Kind != "treasure" || s.Seq != k || seen[s.Player] || g.TimerShare != s.Idx || g.TimerEnd == 0 {
			t.Fatalf("treasure answer %d: %+v (timer for %d, ends %d)", k, s, g.TimerShare, g.TimerEnd)
		}
		seen[s.Player] = true
		end := g.TimerEnd
		g.TimerEnd = end - 1 // the next answer must get a fresh timer
		x.must(x.host("doneShare", Action{}))
		if k < len(g.Order) && g.TimerEnd == end-1 {
			t.Fatal("the timer did not reset for the next person")
		}
	}
	if g.TimerEnd != 0 || g.TimerShare != 0 {
		t.Fatal("the share timer outlived the treasure round")
	}
	x.fails(x.as(p, "sow", own), "sowing where a treasure lies")
	x.must(x.as(p, "take", own))
	if p.Treasure != "lantern" || g.Tiles[tr].Treasure != "" || g.Tiles[tr].Kind != "treasure" {
		t.Fatal("take")
	}
	if e := x.lastEvent("take"); e["treasure"] != "lantern" || e["pid"] != p.ID {
		t.Fatalf("take event %v", e)
	}
	g.Tiles[tr].Treasure = "compass" // pretend another one lies here
	x.turnOf(p)
	x.fails(x.as(p, "take", own), "carrying 2 treasures")
	g.Tiles[tr].Treasure = ""
	x.must(x.as(p, "sow", own)) // once taken, the hex can be sown
	if q.TrustLeft != 10 {
		t.Fatal("trust spent on a treasure moment")
	}
}

// Each of the 9 roles. (Loyalist: TestForestTide; Peacemaker: TestPassing.)
func TestRoles(t *testing.T) {
	x := newTable(t, 9, func(i int) []int { return []int{i + 1} })
	x.play()
	g := x.g
	reformer, helper, achiever, individualist, investigator := x.ps[0], x.ps[1], x.ps[2], x.ps[3], x.ps[4]
	enthusiast, challenger, plain := x.ps[6], x.ps[7], x.ps[8]

	// Reformer: clears 2 layers, 1 action even in rain.
	h := ringHexes(3, 2)[1]
	g.Tiles[h] = &Tile{Up: true, Kind: "empty", Leaves: 2}
	reformer.Pos = Adj[h][0]
	g.Weather[2] = "rain"
	g.Weather[Board[reformer.Pos].Sector] = "rain"
	x.turnOf(reformer)
	x.must(x.as(reformer, "clear", Action{Hex: h}))
	if g.Tiles[h].Leaves != 0 || g.Turn.Actions != 1 {
		t.Fatalf("Reformer clear: leaves %d actions %d", g.Tiles[h].Leaves, g.Turn.Actions)
	}
	x.calm()

	// Helper: Water and Tend a neighbour; Clear works on a neighbour for anyone.
	n := ringHexes(3, 1)[1]
	g.Tiles[n] = &Tile{Up: true, Kind: "empty", Stage: 2}
	helper.Pos = Adj[n][0]
	x.turnOf(helper)
	x.must(x.as(helper, "water", Action{Hex: n}))
	x.must(x.as(helper, "tend", Action{Hex: n}))
	if g.Tiles[n].Stage != 4 {
		t.Fatalf("Helper: stage %d", g.Tiles[n].Stage)
	}
	x.turnOf(helper)
	x.fails(x.as(helper, "water", Action{Hex: ringHexes(4, 4)[0]}), "a Helper watering far away")

	// Achiever: sows straight to Sprout.
	a := ringHexes(4, 5)[3]
	g.Tiles[a] = &Tile{Up: true, Kind: "empty"}
	achiever.Pos = a
	x.turnOf(achiever)
	x.must(x.as(achiever, "sow", own))
	if g.Tiles[a].Stage != 2 || StageNames[2] != "Sprout" || !strings.HasSuffix(g.Log[len(g.Log)-1], "springs up as a Sprout.") {
		t.Fatalf("Achiever sow: stage %d log %q", g.Tiles[a].Stage, g.Log[len(g.Log)-1])
	}
	if Roles[2].Text != "After you Sow, the hex grows straight to Sprout." {
		t.Fatalf("Achiever text %q", Roles[2].Text)
	}

	// Individualist: Explore also peeks at a hidden neighbour, privately.
	in := ringHexes(3, 4)[1]
	individualist.Pos = in
	var nb int
	for _, j := range Adj[in] {
		if j != 0 {
			nb = j
		}
	}
	for _, j := range Adj[in] {
		if j != nb && j != 0 {
			g.Tiles[j].Up = true // only nb stays hidden
		}
	}
	g.Tiles[nb].Kind = "spring"
	x.turnOf(individualist)
	x.must(x.as(individualist, "explore", own))
	if len(individualist.Peeks) != 1 || individualist.Peeks[0].Hex != nb || individualist.Peeks[0].Kind != "spring" {
		t.Fatalf("peeks %+v", individualist.Peeks)
	}
	for _, e := range g.Events {
		if e["type"] == "peek" || e["kind"] != nil {
			t.Fatalf("a private peek leaked into events: %v", e)
		}
	}
	mine := buildView(g, individualist.ID, individualist.Secret, "")["me"].(map[string]any)
	if pk := mine["peeks"].([]Peek); len(pk) != 1 || pk[0].Kind != "spring" {
		t.Fatalf("me.peeks %v", mine["peeks"])
	}
	other := buildView(g, plain.ID, plain.Secret, "")["me"].(map[string]any)
	if len(other["peeks"].([]Peek)) != 0 || other["breath"] != nil {
		t.Fatal("another player sees the peek or the breath")
	}

	// Investigator: sees the next Breath; once a round, one sector's next weather.
	g.Breath = []Drift{{From: 40, To: inwardOf(40)}}
	g.NextWeather[3] = "rain"
	mv := buildView(g, investigator.ID, investigator.Secret, "")["me"].(map[string]any)
	if b := mv["breath"].([]Drift); len(b) != 1 || b[0].From != 40 {
		t.Fatalf("Investigator breath %v", mv["breath"])
	}
	x.fails(x.phone(investigator, "investigate", Action{N: 3}), "investigating on someone else's turn")
	x.turnOf(investigator)
	x.fails(x.phone(plain, "investigate", Action{N: 3}), "a non-Investigator investigating")
	x.fails(x.as(investigator, "investigate", Action{N: 3}), "the Keeper investigating")
	x.fails(x.phone(investigator, "investigate", Action{N: 6}), "investigating sector 6")
	x.must(x.phone(investigator, "investigate", Action{N: 3}))
	x.fails(x.phone(investigator, "investigate", Action{N: 2}), "investigating twice a round")
	mv = buildView(g, investigator.ID, investigator.Secret, "")["me"].(map[string]any)
	if wp, ok := mv["weatherPeek"].(map[string]any); !ok || wp["sector"] != 3 || wp["weather"] != "rain" {
		t.Fatalf("weatherPeek %v", mv["weatherPeek"])
	}
	if !buildView(g, "", "", "host")["turn"].(map[string]any)["investigated"].(bool) {
		t.Fatal("turn.investigated")
	}
	x.endRound()
	mv = buildView(g, investigator.ID, investigator.Secret, "")["me"].(map[string]any)
	if mv["weatherPeek"] != nil {
		t.Fatal("last round's weather look is still shown")
	}
	x.calm()

	// Enthusiast: one 2-hex move a turn, not through or into fog or sealed hexes.
	start := ringHexes(4, 0)[2]
	two := -1
	for _, m := range Adj[start] {
		for _, j := range Adj[m] {
			if j != start && !neighbours(start, j) && Board[j].Ring == 3 {
				two = j
			}
		}
	}
	enthusiast.Pos = start
	x.turnOf(enthusiast)
	g.Weather[Board[start].Sector] = "fog"
	x.fails(x.as(enthusiast, "move", Action{Hex: two}), "a 2-hex move out of fog")
	x.calm()
	x.must(x.as(enthusiast, "move", Action{Hex: two}))
	if enthusiast.Pos != two || !g.Turn.EnthusiastUsed {
		t.Fatal("Enthusiast 2-hex move")
	}
	x.must(x.host("doneShare", Action{})) // the ring card
	back := start
	x.fails(x.as(enthusiast, "move", Action{Hex: back}), "a second 2-hex move")
	x.fails(x.as(plain, "move", Action{Hex: back}), "a plain 2-hex move")
	plain.Pos = start
	x.turnOf(plain)
	x.fails(x.as(plain, "move", Action{Hex: two}), "a 2-hex move without the Enthusiast")

	// Challenger: enters sealed hexes and brings a teammate from the same hex.
	c := ringHexes(4, 3)[1]
	var sealedNb int
	for _, j := range Adj[c] {
		if Board[j].Ring == 3 {
			sealedNb = j
		}
	}
	g.Tiles[sealedNb].Leaves = 2
	challenger.Pos, plain.Pos = c, c
	x.turnOf(plain)
	x.fails(x.as(plain, "move", Action{Hex: sealedNb}), "a plain player entering a sealed hex")
	x.fails(x.as(plain, "move", Action{Hex: sealedNb, Target: challenger.ID}), "a plain player bringing a teammate")
	x.turnOf(challenger)
	x.fails(x.as(challenger, "move", Action{Hex: sealedNb, Target: helper.ID}), "bringing a teammate from another hex")
	x.fails(x.as(challenger, "move", Action{Hex: sealedNb, Target: challenger.ID}), "bringing yourself")
	x.must(x.as(challenger, "move", Action{Hex: sealedNb, Target: plain.ID}))
	if challenger.Pos != sealedNb || plain.Pos != sealedNb {
		t.Fatal("Challenger carry")
	}
	if len(g.Shares) != 2 || g.Shares[0].Player != challenger.ID || g.Shares[1].Player != plain.ID || g.Shares[1].Ring != 3 {
		t.Fatalf("both draw their Ring 3 card: %+v", g.Shares)
	}
}

func TestForestTide(t *testing.T) {
	x := newTable(t, 6, func(i int) []int {
		if i == 0 {
			return []int{6} // the Loyalist
		}
		return []int{9}
	})
	x.play()
	g, ps := x.g, x.ps
	g.NextWeather = [6]string{"rain", "sun", "fog", "fog", "fog", "fog"}
	set := func(i int, tl Tile) int { g.Tiles[i] = &tl; return i }

	// Rain in Zealous grows unsealed Seeded and Sprout, nothing else.
	seed := set(ringHexes(3, 0)[0], Tile{Up: true, Kind: "empty", Stage: 1})
	sealedSprout := set(ringHexes(3, 0)[1], Tile{Up: true, Kind: "empty", Stage: 2, Leaves: 2})
	sapling := set(ringHexes(3, 0)[2], Tile{Up: true, Kind: "empty", Stage: 3, Harvested: true})

	// Sun in Excellence dries walkers, except on a Big Tree; never below 0.
	ps[1].Pos = set(ringHexes(4, 1)[0], Tile{Up: true, Kind: "empty"})
	ps[2].Pos = set(ringHexes(4, 1)[1], Tile{Up: true, Kind: "empty", Stage: 4})
	ps[3].Pos, ps[3].Water = ringHexes(4, 1)[2], 0
	ps[4].Pos = 0 // the World Tree has no weather
	ps[5].Pos = ringHexes(4, 2)[0]

	// The Forest Breath.
	hit := set(ringHexes(3, 3)[0], Tile{Up: true, Kind: "empty", Stage: 1})
	guarded := set(ringHexes(3, 4)[1], Tile{Up: true, Kind: "empty", Stage: 3})
	loyal := -1
	for _, j := range Adj[guarded] {
		if j != 0 && Board[j].Sector >= 3 && !neighbours(j, hit) && j != hit {
			loyal = j
		}
	}
	ps[0].Pos = loyal
	sealing := set(ringHexes(2, 5)[0], Tile{Up: true, Kind: "empty", Stage: 2, Leaves: 1})
	already := set(ringHexes(2, 4)[0], Tile{Up: true, Kind: "empty", Leaves: 2})
	big := set(ringHexes(4, 5)[0], Tile{Up: true, Kind: "empty", Stage: 4})
	g.Breath = []Drift{{hit, hit}, {guarded, guarded}, {sealing, sealing}, {already, already}, {big, big}}

	x.turnOf(ps[0])
	x.endRound()
	if g.Tide != 1 || g.Round != 2 || g.Weather != [6]string{"rain", "sun", "fog", "fog", "fog", "fog"} {
		t.Fatalf("tide %d round %d weather %v", g.Tide, g.Round, g.Weather)
	}
	tl := func(i int) Tile { return *g.Tiles[i] }
	if tl(seed).Stage != 2 || tl(sealedSprout).Stage != 2 || tl(sapling).Stage != 3 || tl(sapling).Harvested {
		t.Fatalf("rain: seed %+v sealed %+v sapling %+v", tl(seed), tl(sealedSprout), tl(sapling))
	}
	if ps[1].Water != 3 || ps[2].Water != 4 || ps[3].Water != 0 || ps[4].Water != 4 || ps[5].Water != 4 {
		t.Fatalf("sun: %d %d %d %d %d", ps[1].Water, ps[2].Water, ps[3].Water, ps[4].Water, ps[5].Water)
	}
	if tl(hit).Stage != 0 || tl(hit).Leaves != 1 {
		t.Fatalf("breath hit %+v", tl(hit))
	}
	if tl(guarded).Stage != 3 || tl(guarded).Leaves != 1 {
		t.Fatalf("the Loyalist's neighbour %+v", tl(guarded))
	}
	if tl(sealing).Leaves != 2 || tl(sealing).Stage != 1 || tl(already).Leaves != 2 || tl(big).Stage != 4 || tl(big).Leaves != 1 {
		t.Fatalf("sealing %+v already %+v big %+v", tl(sealing), tl(already), tl(big))
	}
	e := x.lastEvent("tide")
	if e == nil || !slices.Equal(e["growth"].([]int), []int{seed}) || !slices.Equal(e["dry"].([]string), []string{ps[1].ID}) ||
		len(e["leaves"].([]Drift)) != 4 || !slices.Equal(e["sealed"].([]int), []int{sealing}) {
		t.Fatalf("tide event %v", e)
	}
	if len(g.Breath) != 2 {
		t.Fatalf("next breath %d", len(g.Breath))
	}
	// A Loyalist on the hex itself protects it too.
	ps[0].Pos = hit
	g.Tiles[hit].Stage, g.Tiles[hit].Leaves = 2, 0
	g.Breath = []Drift{{hit, hit}}
	x.calm()
	g.Breath = []Drift{{hit, hit}}
	x.endRound()
	if tl(hit).Stage != 2 || tl(hit).Leaves != 1 {
		t.Fatalf("Loyalist on the hex: %+v", tl(hit))
	}
}

// Passing: on the same hex, or along a chain of up to 2 steps through a Peacemaker.
func TestPassing(t *testing.T) {
	x := newTable(t, 6, func(i int) []int {
		if i == 2 {
			return []int{9} // the Peacemaker
		}
		return []int{1}
	})
	x.play()
	g, ps := x.g, x.ps
	giver, recv, peace, mid := ps[0], ps[1], ps[2], ps[3]
	line := ringHexes(3, -1)[:4]
	for k := 0; k < 3; k++ {
		if !neighbours(line[k], line[k+1]) || (k < 2 && neighbours(line[k], line[k+2])) {
			t.Fatal("the test line isn't a straight chain")
		}
	}
	for _, p := range ps {
		p.Pos = ringHexes(4, 0)[0] // parked out of the way
	}
	place := func(at ...int) {
		for k, p := range []*Player{giver, recv, peace, mid} {
			p.Pos = at[k]
		}
	}
	pass := func(item string) error {
		return x.as(giver, "pass", Action{Target: recv.ID, Text: item})
	}
	x.turnOf(ps[4]) // passing is free and any time, not only on your turn

	park := ringHexes(4, 0)[0]
	place(line[0], line[0], park, park)
	x.must(pass("water"))
	if giver.Water != 3 || recv.Water != 5 {
		t.Fatal("pass water")
	}
	if e := x.lastEvent("pass"); e["from"] != giver.ID || e["to"] != recv.ID || e["item"] != "water" {
		t.Fatalf("pass event %v", e)
	}
	x.fails(pass("water"), "passing water to someone full")
	x.fails(pass("gold"), "passing gold")
	x.fails(x.as(giver, "pass", Action{Target: giver.ID, Text: "water"}), "passing to yourself")
	recv.Water = 2
	place(line[0], line[1], park, park)
	x.fails(pass("water"), "passing to a neighbour without a Peacemaker")
	place(line[0], line[1], line[1], park)
	x.must(pass("water")) // neighbours, Peacemaker on the receiver's hex
	place(line[0], line[2], line[1], park)
	x.must(pass("water")) // a chain through the Peacemaker in the middle
	place(line[0], line[2], line[2], line[1])
	x.must(pass("water")) // a chain with the Peacemaker at the end
	place(line[0], line[2], line[2], park)
	x.fails(pass("water"), "a chain over an empty middle hex")
	place(line[0], line[3], line[2], line[1])
	x.fails(pass("water"), "a chain 3 steps long")

	// Limits: water ≤ 5, fruit ≤ 2, treasure ≤ 1, and you need what you pass.
	place(line[0], line[0], park, park)
	giver.Water = 0
	x.fails(pass("water"), "passing water you don't have")
	x.fails(pass("fruit"), "passing fruit you don't have")
	giver.Fruit, recv.Fruit = 1, 2
	x.fails(pass("fruit"), "a third fruit")
	recv.Fruit = 1
	x.must(pass("fruit"))
	giver.Treasure, recv.Treasure = "rope", "compass"
	x.fails(pass("treasure"), "a second treasure")
	recv.Treasure = ""
	x.must(pass("treasure"))
	if recv.Treasure != "rope" || giver.Treasure != "" || recv.Fruit != 2 {
		t.Fatal("pass fruit and treasure")
	}

	// A share blocks passing.
	g.Shares = append(g.Shares, &Share{Kind: "why", Player: mid.ID, Trusted: map[string]bool{}})
	recv.Water = 1
	giver.Water = 3
	x.fails(pass("water"), "passing during a share")
	x.must(x.host("doneShare", Action{}))

	// Passing to someone on the World Tree places it there at once.
	ring1 := ringHexes(1, -1)[0]
	giver.Fruit, giver.Treasure = 1, ""
	recv.Fruit, recv.Treasure, recv.Placed = 0, "", 0
	place(ring1, 0, ring1, park)
	x.must(pass("fruit"))
	if recv.Placed != 1 || recv.Fruit != 0 || g.openShare() == nil || g.openShare().Kind != "heartwood" || g.openShare().Player != recv.ID {
		t.Fatalf("fruit passed onto the World Tree: placed %d share %+v", recv.Placed, g.openShare())
	}
	x.must(x.host("doneShare", Action{}))

	// Passing outside the turn phase is refused.
	g.Phase = PhaseGuess
	x.fails(pass("water"), "passing in the finale")
}

// Standing on the World Tree places everything at once; fruit brings one question.
func TestAutoPlace(t *testing.T) {
	x := newTable(t, 6, func(i int) []int { return []int{1} })
	x.play()
	g, ps := x.g, x.ps
	ring1 := ringHexes(1, -1)
	p := ps[0]
	p.Pos, p.Fruit, p.Treasure = ring1[0], 2, "compass"
	x.turnOf(p)
	x.must(x.as(p, "move", Action{Hex: 0}))
	if p.Placed != 2 || p.Fruit != 0 || p.Treasure != "" || !slices.Equal(g.Placed, []string{"compass"}) {
		t.Fatalf("placed %d fruit %d treasure %q placed %v", p.Placed, p.Fruit, p.Treasure, g.Placed)
	}
	if len(g.Shares) != 1 || g.Shares[0].Kind != "heartwood" || !slices.Contains(HeartwoodCards, g.Shares[0].Prompt) {
		t.Fatalf("one Heartwood question for 2 fruit: %+v", g.Shares)
	}
	if e := x.lastEvent("place"); e["pid"] != p.ID || e["fruit"] != 2 || e["treasure"] != "compass" {
		t.Fatalf("place event %v", e)
	}
	x.must(x.host("doneShare", Action{}))

	// A treasure alone: no question.
	q := ps[1]
	q.Pos, q.Treasure = ring1[1], "lantern"
	x.turnOf(q)
	x.must(x.as(q, "move", Action{Hex: 0}))
	if len(g.Shares) != 0 || q.Placed != 0 || len(g.Placed) != 2 {
		t.Fatalf("shares %d placed %v", len(g.Shares), g.Placed)
	}
	r := ps[2]
	r.Pos, r.Treasure = ring1[2], "rope"
	x.turnOf(r)
	x.must(x.as(r, "move", Action{Hex: 0}))
	if g.Log[len(g.Log)-1] != HeartComplete || len(g.goals().Treasures) != 3 {
		t.Fatalf("log %q", g.Log[len(g.Log)-1])
	}
	// Walking off and back with nothing: nothing happens.
	x.turnOf(r)
	x.must(x.as(r, "move", Action{Hex: ring1[2]}))
	x.must(x.as(r, "move", Action{Hex: 0}))
	if len(g.Shares) != 0 || g.Phase != PhaseTurn {
		t.Fatal("empty-handed visit placed something")
	}
}

// Ring cards: the first time each player moves inward into Ring 3, 2 and 1.
func TestRingCards(t *testing.T) {
	x := newTable(t, 6, func(i int) []int { return []int{1} })
	x.play()
	g, p := x.g, x.ps[0]
	r4 := p.Pos
	r3 := inwardOf(r4)
	x.turnOf(p)
	x.must(x.as(p, "move", Action{Hex: r3}))
	s := g.openShare()
	if s == nil || s.Kind != "ring" || s.Ring != 3 || !slices.Contains(RingDecks[3], s.Prompt) || !p.Reached[2] {
		t.Fatalf("ring 3 card %+v", s)
	}
	if v := buildView(g, "", "", "host")["share"].(map[string]any); v["sub"] != 3 {
		t.Fatalf("view share sub %v", v["sub"])
	}
	x.must(x.host("doneShare", Action{}))
	x.must(x.as(p, "move", Action{Hex: r4}))
	x.turnOf(p)
	x.must(x.as(p, "move", Action{Hex: r3}))
	if len(g.Shares) != 0 {
		t.Fatal("a second Ring 3 card")
	}
	var side int
	for _, j := range Adj[r3] {
		if Board[j].Ring == 3 {
			side = j
		}
	}
	x.must(x.as(p, "move", Action{Hex: side}))
	if len(g.Shares) != 0 {
		t.Fatal("a card for moving sideways")
	}
	x.turnOf(p)
	r2 := inwardOf(side)
	x.must(x.as(p, "move", Action{Hex: r2}))
	if s := g.openShare(); s == nil || s.Ring != 2 || !slices.Contains(RingDecks[2], s.Prompt) {
		t.Fatalf("ring 2 card %+v", s)
	}
	x.must(x.host("doneShare", Action{}))
	x.must(x.as(p, "move", Action{Hex: inwardOf(r2)}))
	if s := g.openShare(); s == nil || s.Ring != 1 || !slices.Contains(RingDecks[1], s.Prompt) {
		t.Fatalf("ring 1 card %+v", s)
	}
	if p.Reached != [3]bool{true, true, true} {
		t.Fatalf("reached %v", p.Reached)
	}
}

func TestLoseWhenEveryoneIsDry(t *testing.T) {
	x := newTable(t, 6, func(i int) []int { return []int{1} })
	x.play()
	g, ps := x.g, x.ps
	h := ringHexes(3, 0)[0]
	g.Tiles[h] = &Tile{Up: true, Kind: "empty", Stage: 1}
	for _, p := range ps {
		p.Water = 0
	}
	ps[0].Pos, ps[0].Water = h, 1
	x.turnOf(ps[0])
	x.must(x.as(ps[0], "water", own))
	if g.Result != "lost" || g.Phase != PhaseGuess || x.lastEvent("dry") == nil || g.EndedAt == 0 {
		t.Fatalf("result %q phase %s ended at %d", g.Result, g.Phase, g.EndedAt)
	}

	// The sun can do it too, at the Tide.
	y := newTable(t, 6, func(i int) []int { return []int{1} })
	y.play()
	for _, p := range y.ps {
		p.Water = 1
		p.Pos = ringHexes(4, 2)[0]
	}
	y.g.NextWeather[2] = "sun"
	y.endRound()
	if y.g.Result != "lost" || y.g.Phase != PhaseGuess || y.g.Turn != nil {
		t.Fatalf("result %q phase %s", y.g.Result, y.g.Phase)
	}
	// Someone with water left keeps the game going.
	z := newTable(t, 6, func(i int) []int { return []int{1} })
	z.play()
	for _, p := range z.ps[1:] {
		p.Water = 0
	}
	z.endRound()
	if z.g.Phase != PhaseTurn {
		t.Fatal("lost with water left")
	}
}

// The view has exactly the contract's shape, and hides what it should.
func TestViewShape(t *testing.T) {
	x := newTable(t, 6, func(i int) []int { return []int{5} })
	x.play()
	g, p := x.g, x.ps[0]
	g.Tiles[5] = &Tile{Kind: "spring"}
	g.Tiles[6] = &Tile{Kind: "treasure", Treasure: "rope"}
	x.turnOf(p)
	x.must(x.as(p, "move", Action{Hex: inwardOf(p.Pos)})) // opens a ring card
	for range 100 {
		g.event("clear", Event{"hex": 1})
	}
	roundTrip := func(v map[string]any) map[string]any {
		b, err := json.Marshal(v)
		x.must(err)
		var out map[string]any
		x.must(json.Unmarshal(b, &out))
		return out
	}
	keys := func(m map[string]any, want ...string) {
		t.Helper()
		for _, k := range want {
			if _, ok := m[k]; !ok {
				t.Fatalf("missing %q in %v", k, m)
			}
		}
	}
	hv := roundTrip(buildView(g, "", "", "host"))
	keys(hv, "code", "phase", "version", "now", "isHost", "lan", "colors", "values", "roles", "treasures",
		"minPlayers", "maxPlayers", "actionsPerTurn", "round", "tide", "result", "hexes", "tiles", "weather",
		"breathCount", "players", "order", "current", "turn", "share", "goals", "events", "log", "timerEnd",
		"timerLabel", "tribute", "recognition", "me", "stages", "ringDecks")
	if fmt.Sprint(hv["stages"]) != "[ Seeded Sprout Sapling Big Tree]" || fmt.Sprint(hv["ringDecks"]) != "[ Deep Story Light]" {
		t.Fatalf("stages %v ringDecks %v", hv["stages"], hv["ringDecks"])
	}
	if hv["isHost"] != true || hv["me"] != nil || hv["minPlayers"] != 6.0 || hv["maxPlayers"] != 12.0 || hv["actionsPerTurn"] != 2.0 {
		t.Fatalf("host view basics %v %v %v", hv["isHost"], hv["me"], hv["minPlayers"])
	}
	hexes, tiles := hv["hexes"].([]any), hv["tiles"].([]any)
	if len(hexes) != 61 || len(tiles) != 61 || tiles[0] != nil {
		t.Fatal("hexes/tiles")
	}
	keys(hexes[1].(map[string]any), "q", "r", "ring", "sector")
	t5 := tiles[5].(map[string]any)
	keys(t5, "up", "kind", "treasure", "stage", "leaves", "harvested")
	if t5["kind"] != "" || tiles[6].(map[string]any)["treasure"] != "" {
		t.Fatal("a face-down tile shows what it is")
	}
	if len(hv["weather"].([]any)) != 6 || len(hv["roles"].([]any)) != 9 || len(hv["treasures"].([]any)) != 3 {
		t.Fatal("weather/roles/treasures")
	}
	keys(hv["treasures"].([]any)[0].(map[string]any), "id", "icon", "name", "meaning")
	keys(hv["roles"].([]any)[0].(map[string]any), "type", "name", "text")
	pl := hv["players"].([]any)[0].(map[string]any)
	keys(pl, "id", "name", "color", "value", "types", "pos", "water", "fruit", "treasure", "placed", "reached",
		"trustLeft", "guessed", "photo")
	if pl["photo"] != 0.0 {
		t.Fatalf("photo %v", pl["photo"])
	}
	if _, secret := pl["secret"]; secret || len(pl["reached"].([]any)) != 3 {
		t.Fatal("player secret leaked, or reached is not 3 bools")
	}
	keys(hv["turn"].(map[string]any), "player", "actions", "enthusiastUsed", "investigated")
	sh := hv["share"].(map[string]any)
	keys(sh, "idx", "kind", "player", "prompt", "sub", "trustCount", "iGave")
	if sh["kind"] != "ring" || sh["sub"] != 3.0 {
		t.Fatalf("share %v", sh)
	}
	keys(hv["goals"].(map[string]any), "trees", "placedPlayers", "players", "treasures", "onTree")
	evs := hv["events"].([]any)
	if len(evs) != 80 || evs[79].(map[string]any)["id"].(float64) != float64(g.EventSeq) {
		t.Fatalf("events %d", len(evs))
	}
	for k := 1; k < len(evs); k++ {
		if evs[k].(map[string]any)["id"].(float64) <= evs[k-1].(map[string]any)["id"].(float64) {
			t.Fatal("event ids don't increase")
		}
	}
	if hv["current"] != p.ID {
		t.Fatal("current")
	}

	// A player's own view: me, with the Investigator's breath.
	q := x.ps[1]
	pv := roundTrip(buildView(g, q.ID, q.Secret, ""))
	me := pv["me"].(map[string]any)
	keys(me, "id", "target", "targetName", "guess", "peeks", "breath")
	if pv["isHost"] != false || me["targetName"] == "" || pv["share"].(map[string]any)["iGave"] != false {
		t.Fatal("player view")
	}
	x.must(x.phone(q, "trust", Action{}))
	if pv := roundTrip(buildView(g, q.ID, q.Secret, "")); pv["share"].(map[string]any)["iGave"] != true || pv["share"].(map[string]any)["trustCount"] != 1.0 {
		t.Fatal("iGave/trustCount")
	}
	if bad := roundTrip(buildView(g, q.ID, "wrong", "")); bad["me"] != nil {
		t.Fatal("me without the secret")
	}
	if lobby := roundTrip(buildView(NewGame("L", "h"), "", "", "")); lobby["turn"] != nil || len(lobby["order"].([]any)) != 0 || lobby["phase"] != "lobby" {
		t.Fatal("lobby view")
	}

	// GET /api/cards
	rec := httptest.NewRecorder()
	cards(rec, httptest.NewRequest("GET", "/api/cards", nil))
	var c map[string]any
	x.must(json.Unmarshal(rec.Body.Bytes(), &c))
	keys(c, "values", "ringDecks", "heartwood", "harvest", "treasures", "roles", "stages", "ringDeckNames")
	if fmt.Sprint(c["stages"]) != "[ Seeded Sprout Sapling Big Tree]" || fmt.Sprint(c["ringDeckNames"]) != "[ Deep Story Light]" {
		t.Fatalf("cards stages %v ringDeckNames %v", c["stages"], c["ringDeckNames"])
	}
	rd := c["ringDecks"].(map[string]any)
	for _, r := range []string{"1", "2", "3"} {
		if len(rd[r].([]any)) != 30 {
			t.Fatalf("ring deck %s has %d", r, len(rd[r].([]any)))
		}
	}
	if len(c["heartwood"].([]any)) < 14 || !strings.Contains(c["harvest"].(string), "{value}") ||
		len(c["treasures"].([]any)) != 3 || len(c["roles"].([]any)) != 9 {
		t.Fatal("cards")
	}
	keys(c["treasures"].([]any)[0].(map[string]any), "id", "icon", "name", "meaning", "question")
	if Roles[8].Text != "Teammates in a chain up to 2 hexes long through you can pass things to each other." {
		t.Fatal("Peacemaker text")
	}
}

// Saved games from the old rules are skipped on load.
func TestLoadSkipsOldRules(t *testing.T) {
	db, err := openDB(filepath.Join(t.TempDir(), "hw.db"))
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{db: db, games: map[string]*Game{}, subs: map[string]map[chan struct{}]bool{}}
	_, err = db.Exec(`INSERT INTO games(code,state,updated_at) VALUES('OLD1', '{"code":"OLD1","phase":"scores","season":3,"tokens":{"3":{"kind":"path"}}}', 0)`)
	if err != nil {
		t.Fatal(err)
	}
	g := NewGame("NEW1", "h")
	if err := s.save(g); err != nil {
		t.Fatal(err)
	}
	if err := s.load(); err != nil {
		t.Fatal(err)
	}
	if s.games["OLD1"] != nil || s.games["NEW1"] == nil || len(s.games["NEW1"].Tiles) != 61 {
		t.Fatalf("loaded %v", s.games)
	}
}

// The Keeper can close the game early during play; it goes to the finale.
func TestEndGameEarly(t *testing.T) {
	x := newTable(t, 6, nil)
	x.must(x.host("start", Action{}))
	x.fails(x.host("endGame", Action{}), "ending the game while entering")
	x.play2()
	g := x.g
	x.fails(x.phone(x.ps[0], "endGame", Action{}), "a player ending the game")
	x.fails(x.as(x.ps[0], "endGame", Action{}), "the Keeper ending the game as a player")
	g.Shares = append(g.Shares, &Share{Kind: "why", Player: x.ps[0].ID, Trusted: map[string]bool{}})
	x.fails(x.host("endGame", Action{}), "ending the game during a share")
	x.must(x.host("doneShare", Action{}))
	if g.EndedAt != 0 {
		t.Fatal("ended at is set before the end")
	}
	x.must(x.host("endGame", Action{}))
	if g.Result != "ended" || g.Phase != PhaseGuess || g.Turn != nil || g.Log[len(g.Log)-2] != "The Keeper closed the game early." || g.EndedAt == 0 {
		t.Fatalf("result %q phase %s log %v ended at %d", g.Result, g.Phase, g.Log[len(g.Log)-2:], g.EndedAt)
	}
	if buildView(g, "", "", "host")["result"] != "ended" {
		t.Fatal("view result")
	}
	x.fails(x.host("endGame", Action{}), "ending the game twice")
	x.must(x.host("startChain", Action{}))
	for range 6 {
		x.must(x.host("nextTribute", Action{}))
	}
	if g.Phase != PhaseEnd || len(g.Recognition()) != 6 {
		t.Fatal("finale after closing early")
	}
}
