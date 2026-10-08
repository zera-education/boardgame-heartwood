package main

// Heartwood: the rules engine. A cooperative game for 6 to 12 players: the
// team wakes the forest (a Big Tree in every value, everyone places a fruit, the
// three treasures on the World Tree, everyone gathered there), or everyone runs
// out of water. Players say what they do and the Keeper taps it on the big
// screen; trust acorns and the Secret Owl stay on the phones.

import (
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"
)

// RulesVersion marks a saved game as made under these rules; load() skips
// games saved under older ones, which don't fit these structs.
const RulesVersion = "world-tree-1"

const (
	PhaseLobby = "lobby"
	PhaseBrief = "brief" // the Keeper briefs the team with slides on the big screen
	PhaseEnter = "enter" // each player picks a value and an outer hex, and says why
	PhaseTurn  = "turn"
	PhaseGuess = "guess" // finale: guess your Secret Owl
	PhaseChain = "chain" // finale: the tribute chain
	PhaseEnd   = "end"
)

const (
	MinPlayers     = 6
	MaxPlayers     = 12
	ActionsPerTurn = 2
	StartWater     = 4
	MaxWater       = 5
	MaxFruit       = 2
	StartTrust     = 10
	SpringCount    = 4
	MaxBreath      = 6
	Rings          = 4
)

// Enneagram types, as role numbers.
const (
	Reformer = iota + 1
	Helper
	Achiever
	Individualist
	Investigator
	Loyalist
	Enthusiast
	Challenger
	Peacemaker
)

// StageNames names the growth stages by number (0 = nothing growing).
var StageNames = [5]string{"", "Seeded", "Sprout", "Sapling", "Big Tree"}

var WeatherKinds = [3]string{"sun", "rain", "fog"}

type Hex struct {
	Q      int `json:"q"`
	R      int `json:"r"`
	Ring   int `json:"ring"`
	Sector int `json:"sector"` // the value, 0..5; -1 for the World Tree
}

var dirs = [6][2]int{{1, 0}, {1, -1}, {0, -1}, {-1, 0}, {-1, 1}, {0, 1}}

// buildHexes lays out the World Tree and its rings, ring by ring.
func buildHexes(rings int) []Hex {
	hs := []Hex{{0, 0, 0, -1}}
	for k := 1; k <= rings; k++ {
		q, r := dirs[4][0]*k, dirs[4][1]*k
		for side := 0; side < 6; side++ {
			for s := 0; s < k; s++ {
				// Values go clockwise from the top, so the forest reads Z-E-R-A-O-S.
				hs = append(hs, Hex{q, r, k, (3 - side + 6) % 6})
				q += dirs[side][0]
				r += dirs[side][1]
			}
		}
	}
	return hs
}

func buildAdj(hs []Hex) [][]int {
	at := map[[2]int]int{}
	for i, h := range hs {
		at[[2]int{h.Q, h.R}] = i
	}
	adj := make([][]int, len(hs))
	for i, h := range hs {
		for _, d := range dirs {
			if j, ok := at[[2]int{h.Q + d[0], h.R + d[1]}]; ok {
				adj[i] = append(adj[i], j)
			}
		}
	}
	return adj
}

// Board is the same for every game: 61 hexes, index 0 the World Tree. Adj
// lists each hex's neighbours.
var (
	Board = buildHexes(Rings)
	Adj   = buildAdj(Board)
)

func neighbours(a, b int) bool {
	if a < 0 || a >= len(Adj) {
		return false
	}
	for _, j := range Adj[a] {
		if j == b {
			return true
		}
	}
	return false
}

// Tile is what lies on a hex (every hex but the World Tree).
type Tile struct {
	Up        bool   `json:"up"`             // explored
	Kind      string `json:"kind"`           // empty, spring or treasure
	Treasure  string `json:"treasure"`       // the treasure still lying here; "" once taken
	Stage     int    `json:"stage"`          // 0 none, 1 Seeded, 2 Sprout, 3 Sapling, 4 Big Tree
	Leaves    int    `json:"leaves"`         // dead leaves; 2 = sealed
	Harvested bool   `json:"harvested"`      // since the last Forest Tide
	Hint      bool   `json:"hint,omitempty"` // face down, and an Individualist sensed whether something lies here
}

// Drift is one gust of the Forest Breath: dead leaves picked up at From land on
// To. From is -1 for a gust from beyond the forest edge (onto Ring 4).
type Drift struct {
	From int `json:"from"`
	To   int `json:"to"`
}

// Peek is a private look that goes only to one player's phone: an
// Investigator's look at a sector's next weather (games before 2026-10-09 also
// kept an Individualist's peek at a hidden tile here).
type Peek struct {
	Type    string `json:"type"` // "tile" or "weather"
	Hex     int    `json:"hex"`  // tile: the hex peeked at (-1 for weather)
	Kind    string `json:"kind,omitempty"`
	Sector  int    `json:"sector"`
	Weather string `json:"weather,omitempty"`
	Round   int    `json:"round"`
	Tide    int    `json:"tide"` // the Forest Tide count when it was seen
}

type Player struct {
	ID        string         `json:"id"`
	Secret    string         `json:"secret"`
	Name      string         `json:"name"`
	Color     string         `json:"color"`
	Value     int            `json:"value"` // the ZERAOS value they stand for; -1 until they enter
	Types     []int          `json:"types"`
	Pos       int            `json:"pos"` // -1 until they enter
	Water     int            `json:"water"`
	Fruit     int            `json:"fruit"`
	Treasure  string         `json:"treasure"` // the treasure they carry, or ""
	Placed    int            `json:"placed"`   // fruit placed on the World Tree
	Reached   [3]bool        `json:"reached"`  // Rings 1, 2, 3: ring card drawn
	TrustLeft int            `json:"trustLeft"`
	Pot       map[string]int `json:"pot"` // trust acorns received, by giver
	Target    string         `json:"target"`
	Guess     string         `json:"guess"`
	Peeks     []Peek         `json:"peeks"`
	Photo     int            `json:"photo"` // 0 = no photo; a new number on every change (the photo is in the photos table)
	// What they did, for the recognition: treasures turned up exploring and
	// picked up, fruit harvested, springs found.
	Found     []string `json:"found,omitempty"`
	Took      []string `json:"took,omitempty"`
	Harvested int      `json:"harvested,omitempty"`
	Springs   int      `json:"springs,omitempty"`
}

func (p *Player) has(t int) bool {
	for _, x := range p.Types {
		if x == t {
			return true
		}
	}
	return false
}

type TurnState struct {
	Player         string `json:"player"`
	Actions        int    `json:"actions"`
	EnthusiastUsed bool   `json:"enthusiastUsed"`
	Investigated   bool   `json:"investigated"`
}

// Share is one person speaking (or, for a treasure, the whole table). Shares
// queue; while one is open no play action is taken.
type Share struct {
	Idx     int             `json:"idx"`
	Kind    string          `json:"kind"` // why, ring, harvest, heartwood, treasure
	Player  string          `json:"player"`
	Prompt  string          `json:"prompt"`
	Sub     string          `json:"sub"`  // tagline, or treasure id
	Ring    int             `json:"ring"` // ring cards: the ring
	Trusted map[string]bool `json:"trusted"`
	// A treasure's question goes round the whole table, one share per person:
	// By found it, and this is answer Seq of Of.
	By  string `json:"by,omitempty"`
	Seq int    `json:"seq,omitempty"`
	Of  int    `json:"of,omitempty"`
	// Was is the share this one replaced when its player said "not this one"
	// and drew another card (another).
	Was int `json:"was,omitempty"`
}

// Event is one thing that happened, for the Keeper screen's animation.
type Event map[string]any

type Game struct {
	Rules       string               `json:"rules"`
	Code        string               `json:"code"`
	HostSecret  string               `json:"hostSecret"`
	Phase       string               `json:"phase"`
	Players     []*Player            `json:"players"`
	Order       []string             `json:"order"`
	TurnIdx     int                  `json:"turnIdx"`
	Round       int                  `json:"round"`
	Tide        int                  `json:"tide"`
	Result      string               `json:"result"`    // "", won, lost, ended (closed early by the Keeper)
	EndedAt     int64                `json:"endedAt"`   // unix seconds when Result was set; photos go 2 hours later
	CreatedAt   int64                `json:"createdAt"` // unix seconds; 0 for games made before it was noted
	Tiles       []*Tile              `json:"tiles"`     // index 0 (the World Tree) is nil
	Weather     [6]string            `json:"weather"`
	NextWeather [6]string            `json:"nextWeather"`
	Breath      []Drift              `json:"breath"` // the next Forest Breath, rolled ahead
	Turn        *TurnState           `json:"turn"`
	Shares      []*Share             `json:"shares"` // the queue; [0] is open
	ShareSeq    int                  `json:"shareSeq"`
	Placed      []string             `json:"placed"` // treasures on the World Tree
	Slide       int                  `json:"slide"`  // the briefing slide every screen shows
	Decks       map[string][]int     `json:"decks"`
	Chain       []string             `json:"chain"`
	ChainIdx    int                  `json:"chainIdx"`
	Log         []string             `json:"log"`
	Events      []Event              `json:"events"`
	EventSeq    int                  `json:"eventSeq"`
	TimerEnd    int64                `json:"timerEnd"`
	TimerLabel  string               `json:"timerLabel"`
	TimerShare  int                  `json:"timerShare"` // the share the running timer belongs to (0: none)
	Version     int                  `json:"version"`
	Rejoin      map[string]RejoinPin `json:"rejoin"`
	// Story recording (recording.go): on unless the Keeper switches it off; one
	// Keeper device records.
	NoRecord  bool   `json:"noRecord,omitempty"`
	RecDevice string `json:"recDevice,omitempty"`
	// The Keeper's taps this turn, newest last, to take back a slip (undo.go).
	Undo   []UndoPoint `json:"undo,omitempty"`
	logged int         // trail log lines written since the server loaded the game
	// A sealed find turned up in this game (find.go).
	Found      *Found `json:"found,omitempty"`
	FoundTries int    `json:"foundTries,omitempty"` // explores by its player so far
}

type RejoinPin struct {
	Pid     string `json:"pid"`
	Expires int64  `json:"expires"`
}

type Action struct {
	Pid    string `json:"pid"`
	Secret string `json:"secret"`
	Host   string `json:"host"`
	Type   string `json:"type"`
	Hex    int    `json:"hex"` // -1 when not given: your own hex
	Target string `json:"target"`
	N      int    `json:"n"`
	Text   string `json:"text"`
	Types  []int  `json:"types"`
	As     string `json:"as"` // the player the Keeper acts for
	// Move: the teammates on a Challenger's hex who come along. Left out, all
	// of them come (but nobody is taken off the World Tree).
	Bring []string `json:"bring"`
}

func NewGame(code, hostSecret string) *Game {
	g := &Game{Rules: RulesVersion, Code: code, HostSecret: hostSecret, Phase: PhaseLobby,
		Decks: map[string][]int{}, Placed: []string{}, CreatedAt: time.Now().Unix()}
	g.lay()
	for s := range 6 {
		g.Weather[s] = drawWeather()
	}
	for s := range 6 {
		g.NextWeather[s] = drawWeather()
	}
	g.Breath = g.rollBreath()
	return g
}

// lay deals the 60 face-down tiles: the three treasures on random hexes of
// Rings 1 and 2, four springs anywhere else, the rest empty.
func (g *Game) lay() {
	g.Tiles = make([]*Tile, len(Board))
	for i := 1; i < len(Board); i++ {
		g.Tiles[i] = &Tile{Kind: "empty"}
	}
	var inner, rest []int
	for i := 1; i < len(Board); i++ {
		if Board[i].Ring <= 2 {
			inner = append(inner, i)
		}
	}
	rand.Shuffle(len(inner), func(i, j int) { inner[i], inner[j] = inner[j], inner[i] })
	ids := rand.Perm(len(Treasures))
	hidden := map[int]bool{}
	for k, id := range ids {
		t := g.Tiles[inner[k]]
		t.Kind, t.Treasure = "treasure", Treasures[id].ID
		hidden[inner[k]] = true
	}
	for i := 1; i < len(Board); i++ {
		if !hidden[i] {
			rest = append(rest, i)
		}
	}
	rand.Shuffle(len(rest), func(i, j int) { rest[i], rest[j] = rest[j], rest[i] })
	for _, i := range rest[:SpringCount] {
		g.Tiles[i].Kind = "spring"
	}
}

func drawWeather() string { return WeatherKinds[rand.Intn(3)] }

// edgeSpots is where the wind can also start outside the forest: one for each
// hex of the ring beyond Ring 4. Gusts from there (From -1) land on Ring 4
// (El, 2026-10-08: the outer ring gets dead leaves too).
const edgeSpots = 6 * (Rings + 1)

// rollBreath decides the next Forest Breath ahead, so the Investigator can see
// it: each gust picks a hex or a spot beyond the forest edge, and its dead
// leaves land one step toward the centre (a Ring 1 hex keeps them; the World
// Tree never gets any). It grows by one every two Tides, up to 6.
func (g *Game) rollBreath() []Drift {
	n := min(MaxBreath, 2+g.Tide/2)
	out := []Drift{}
	var outer []int
	for i, h := range Board {
		if h.Ring == Rings {
			outer = append(outer, i)
		}
	}
	for range n {
		k := rand.Intn(len(Board) - 1 + edgeSpots)
		if k >= len(Board)-1 {
			out = append(out, Drift{From: -1, To: outer[rand.Intn(len(outer))]})
			continue
		}
		from := 1 + k
		var inward []int
		for _, j := range Adj[from] {
			if j != 0 && Board[j].Ring == Board[from].Ring-1 {
				inward = append(inward, j)
			}
		}
		to := from
		if len(inward) > 0 {
			to = inward[rand.Intn(len(inward))]
		}
		out = append(out, Drift{from, to})
	}
	return out
}

func (g *Game) player(id string) *Player {
	for _, p := range g.Players {
		if p.ID == id {
			return p
		}
	}
	return nil
}

func (g *Game) logf(format string, a ...any) {
	g.logged++
	g.Log = append(g.Log, fmt.Sprintf(format, a...))
	if len(g.Log) > 200 {
		g.Log = g.Log[len(g.Log)-200:]
	}
}

// event records something for the Keeper screen to animate. Ids keep
// increasing for the whole game.
func (g *Game) event(typ string, e Event) {
	if e == nil {
		e = Event{}
	}
	g.EventSeq++
	e["id"], e["type"] = g.EventSeq, typ
	g.Events = append(g.Events, e)
	if len(g.Events) > 200 {
		g.Events = g.Events[len(g.Events)-200:]
	}
}

func (g *Game) setTimer(secs int, label string) {
	if secs <= 0 {
		g.TimerEnd, g.TimerLabel = 0, ""
		return
	}
	g.TimerEnd = time.Now().Add(time.Duration(secs) * time.Second).UnixMilli()
	g.TimerLabel = label
}

// draw takes the next card of a deck ("ring1".."ring3", "heartwood"),
// reshuffling it when it runs out.
func deckCards(deck string) []string {
	switch deck {
	case "ring1":
		return RingDecks[1]
	case "ring2":
		return RingDecks[2]
	case "ring3":
		return RingDecks[3]
	}
	return HeartwoodCards
}

func (g *Game) draw(deck string) string {
	cards := deckCards(deck)
	for {
		if len(g.Decks[deck]) == 0 {
			g.Decks[deck] = rand.Perm(len(cards))
		}
		c := g.Decks[deck][0]
		g.Decks[deck] = g.Decks[deck][1:]
		// a deck shuffled before the cards changed can hold numbers past the end: skip them
		if c < len(cards) {
			return cards[c]
		}
	}
}

// wx is the weather on a hex ("" on the World Tree).
func (g *Game) wx(i int) string {
	if i <= 0 || i >= len(Board) {
		return ""
	}
	return g.Weather[Board[i].Sector]
}

func (g *Game) sealed(i int) bool {
	return i > 0 && i < len(g.Tiles) && g.Tiles[i].Leaves >= 2
}

func (g *Game) tile(i int) *Tile {
	if i <= 0 || i >= len(g.Tiles) {
		return nil
	}
	return g.Tiles[i]
}

// ---- lobby ----

func (g *Game) Join(name, color string) (*Player, error) {
	if g.Phase != PhaseLobby {
		return nil, errors.New("the game has already started")
	}
	if len(g.Players) >= MaxPlayers {
		return nil, fmt.Errorf("the game is full (%d players)", MaxPlayers)
	}
	if name == "" {
		return nil, errors.New("enter your name")
	}
	for _, p := range g.Players {
		if p.Color == color {
			return nil, errors.New("that colour is taken")
		}
		if p.Name == name {
			return nil, errors.New("that name is taken")
		}
	}
	p := &Player{ID: randID(6), Secret: randID(16), Name: name, Color: color, Value: -1, Pos: -1,
		Water: StartWater, Pot: map[string]int{}}
	g.Players = append(g.Players, p)
	g.logf("%s joined", name)
	return p, nil
}

// assignPowers is the Keeper giving a player 1 to 3 Enneagram types in the
// lobby; each type's role is always on. An empty list clears them.
func (g *Game) assignPowers(pid string, types []int) error {
	if g.Phase != PhaseLobby {
		return errors.New("types are assigned in the lobby")
	}
	p := g.player(pid)
	if p == nil {
		return errors.New("no such player")
	}
	if len(types) > 3 {
		return errors.New("up to 3 types per player")
	}
	seen := map[int]bool{}
	for _, t := range types {
		if t < 1 || t > 9 || seen[t] {
			return errors.New("choose different types from 1 to 9")
		}
		seen[t] = true
	}
	p.Types = append([]int(nil), types...)
	return nil
}

// randomTypes is the Keeper dealing every player exactly one Enneagram type at
// random, spread evenly: all different up to 9 players, and no type a second
// time before every type has been dealt once. It replaces the types they had.
func (g *Game) randomTypes() error {
	if g.Phase != PhaseLobby {
		return errors.New("types are assigned in the lobby")
	}
	if len(g.Players) == 0 {
		return errors.New("no one has joined yet")
	}
	var bag []int
	for _, p := range g.Players {
		if len(bag) == 0 {
			bag = rand.Perm(9)
		}
		p.Types = []int{bag[0] + 1}
		bag = bag[1:]
	}
	g.logf("The Keeper dealt one Enneagram type to each player at random.")
	return nil
}

func (g *Game) start() error {
	if g.Phase != PhaseLobby {
		return errors.New("already started")
	}
	if len(g.Players) < MinPlayers {
		return fmt.Errorf("need at least %d players", MinPlayers)
	}
	if len(g.Players) > MaxPlayers {
		return fmt.Errorf("at most %d players", MaxPlayers)
	}
	for _, p := range g.Players {
		if len(p.Types) == 0 {
			return fmt.Errorf("give %s at least one Enneagram type", p.Name)
		}
	}
	g.Order = nil
	for _, p := range g.Players {
		g.Order = append(g.Order, p.ID)
		p.TrustLeft = StartTrust
		p.Water = StartWater
	}
	// Secret Owl: one loop through a shuffled order. Each player is the Secret
	// Owl of their Target, watching them quietly all game.
	g.Chain = nil
	perm := rand.Perm(len(g.Players))
	for i, pi := range perm {
		g.Players[pi].Target = g.Players[perm[(i+1)%len(perm)]].ID
		g.Chain = append(g.Chain, g.Players[pi].ID)
	}
	g.Phase = PhaseBrief
	g.Slide = 0
	g.logf("The Keeper briefs the team: our goal, what stands in our way, our powers and the Secret Owl.")
	return nil
}

// MaxSlides caps the briefing slide number (the Keeper screen has fewer).
const MaxSlides = 32

// slide shows briefing slide n on every screen.
func (g *Game) slide(n int) error {
	if g.Phase != PhaseBrief {
		return errors.New("the briefing is over")
	}
	g.Slide = max(0, min(n, MaxSlides-1))
	return nil
}

// begin ends the briefing: the players enter the forest.
func (g *Game) begin() error {
	if g.Phase != PhaseBrief {
		return errors.New("not now")
	}
	g.Phase = PhaseEnter
	g.TurnIdx = 0
	g.logf("The forest sleeps. Each of you: choose the value you stand for, step onto its edge, and say why.")
	return nil
}

// ---- shares and trust ----

func (g *Game) share(kind string, p *Player, prompt, sub string, ring int) {
	g.ShareSeq++
	g.Shares = append(g.Shares, &Share{Idx: g.ShareSeq, Kind: kind, Player: p.ID, Prompt: prompt, Sub: sub,
		Ring: ring, Trusted: map[string]bool{}})
}

// ShareSeconds is the timer that starts by itself when a share opens: the
// opening "why" and each person's one sentence for a treasure. Other shares
// start with no timer (the Keeper can start one).
var ShareSeconds = map[string]int{"why": 10, "treasure": 10}

// syncShareTimer gives every share its own timer: when the open share changes,
// the old share's timer goes, and the new one's starts if its kind has one.
func (g *Game) syncShareTimer() {
	s, idx := g.openShare(), 0
	if s != nil {
		idx = s.Idx
	}
	if idx == g.TimerShare {
		return
	}
	if g.TimerShare != 0 || s != nil {
		g.TimerEnd, g.TimerLabel = 0, ""
	}
	g.TimerShare = idx
	if s != nil && ShareSeconds[s.Kind] > 0 {
		g.setTimer(ShareSeconds[s.Kind], "Share")
	}
}

func (g *Game) openShare() *Share {
	if len(g.Shares) == 0 {
		return nil
	}
	return g.Shares[0]
}

// giveTrust is a listener giving one trust acorn to the person sharing.
func (g *Game) giveTrust(me *Player) error {
	s := g.openShare()
	if s == nil {
		return errors.New("trust is given while someone is sharing")
	}
	if s.Kind == "treasure" {
		return errors.New("everyone answers a treasure's question: no acorns for it")
	}
	if s.Player == me.ID {
		return errors.New("you can't trust your own share")
	}
	if s.Trusted[me.ID] {
		return errors.New("you've already given an acorn for this share")
	}
	if me.TrustLeft < 1 {
		return errors.New("no trust acorns left")
	}
	sharer := g.player(s.Player)
	if sharer == nil {
		return errors.New("no one to trust")
	}
	me.TrustLeft--
	sharer.Pot[me.ID]++
	s.Trusted[me.ID] = true
	return nil
}

func (g *Game) doneShare() error {
	if len(g.Shares) == 0 {
		return errors.New("no one is sharing")
	}
	g.Shares = g.Shares[1:]
	return nil
}

// another is a player saying "not this one" to a ring or Heartwood card: they
// draw another from the same deck, and the card they passed on goes under the
// deck for someone else. The new card is a new share (a fresh recording and
// timer); by is the player asking on their phone, nil for the Keeper.
func (g *Game) another(by *Player) error {
	s := g.openShare()
	if s == nil || (s.Kind != "ring" && s.Kind != "heartwood") {
		return errors.New("only a ring card or a Heartwood card can be swapped")
	}
	if by != nil && by.ID != s.Player {
		return errors.New("only the one sharing can ask for another card")
	}
	deck := "heartwood"
	if s.Kind == "ring" {
		deck = fmt.Sprintf("ring%d", s.Ring)
	}
	cards := deckCards(deck)
	if len(cards) < 2 {
		return errors.New("this deck has no other card")
	}
	old, next := s.Prompt, s.Prompt
	for range 2 * len(cards) {
		if next = g.draw(deck); next != old {
			break
		}
	}
	if next == old {
		return errors.New("this deck has no other card")
	}
	for k, c := range cards {
		if c == old {
			g.Decks[deck] = append(g.Decks[deck], k)
		}
	}
	g.ShareSeq++
	s.Was, s.Idx, s.Prompt = s.Idx, g.ShareSeq, next
	if p := g.player(s.Player); p != nil {
		g.logf("%s draws another card.", p.Name)
	}
	return nil
}

// ---- entering ----

func (g *Game) current() *Player {
	if g.TurnIdx < 0 || g.TurnIdx >= len(g.Order) {
		return nil
	}
	return g.player(g.Order[g.TurnIdx])
}

func (g *Game) enter(me *Player, value, hex int) error {
	if g.Phase != PhaseEnter {
		return errors.New("everyone has entered the forest")
	}
	if cur := g.current(); cur != me {
		return fmt.Errorf("it's %s's turn to enter", cur.Name)
	}
	if value < 0 || value > 5 {
		return errors.New("choose a value")
	}
	if hex < 0 || hex >= len(Board) || Board[hex].Ring != Rings || Board[hex].Sector != value {
		return errors.New("stand on the outer ring of your value's sector")
	}
	me.Value, me.Pos = value, hex
	v := Values[value]
	g.share("why", me, fmt.Sprintf("Why do you stand for %s?", v.Name), v.Tagline, 0)
	g.event("enter", Event{"pid": me.ID, "hex": hex, "value": value})
	g.logf("%s stands for %s.", me.Name, v.Name)
	g.TurnIdx++
	if g.TurnIdx >= len(g.Order) {
		g.Phase = PhaseTurn
		g.TurnIdx, g.Round = 0, 1
		g.newTurn()
		g.logf("Round 1.")
	}
	return nil
}

// ---- turns ----

func (g *Game) newTurn() {
	g.Turn = &TurnState{Player: g.Order[g.TurnIdx], Actions: ActionsPerTurn}
	g.Undo = nil
	g.TimerEnd, g.TimerLabel = 0, ""
}

func (g *Game) spend(n int) error {
	if g.Turn.Actions < n {
		if n > 1 && g.Turn.Actions > 0 {
			return fmt.Errorf("that costs %d actions here", n)
		}
		return errors.New("no actions left this turn")
	}
	g.Turn.Actions -= n
	return nil
}

func (g *Game) canEnter(p *Player, j int) bool {
	return !g.sealed(j) || p.has(Challenger)
}

// doubleStep says whether an Enthusiast may reach `to` with this turn's one
// 2-hex move: not from fog, and neither hex sealed or in fog.
func (g *Game) doubleStep(p *Player, to int) bool {
	if !p.has(Enthusiast) || g.Turn.EnthusiastUsed || g.wx(p.Pos) == "fog" || to == p.Pos {
		return false
	}
	if to < 0 || to >= len(Board) || g.sealed(to) || g.wx(to) == "fog" {
		return false
	}
	for _, m := range Adj[p.Pos] {
		if !g.sealed(m) && g.wx(m) != "fog" && neighbours(m, to) {
			return true
		}
	}
	return false
}

func (g *Game) move(me *Player, to int, bring []string) error {
	if me.Water <= 0 {
		return fmt.Errorf("%s has no water and can't move until someone passes 1", me.Name)
	}
	if to < 0 || to >= len(Board) || to == me.Pos {
		return errors.New("choose a hex to move to")
	}
	one := neighbours(me.Pos, to) && g.canEnter(me, to)
	two := !one && g.doubleStep(me, to)
	if !one && !two {
		if neighbours(me.Pos, to) {
			return errors.New("that hex is sealed by dead leaves: clear it first")
		}
		return errors.New("move to a hex next to you")
	}
	mates, err := g.bring(me, bring)
	if err != nil {
		return err
	}
	if err := g.spend(1); err != nil {
		return err
	}
	if two {
		g.Turn.EnthusiastUsed = true
	}
	from := me.Pos
	movers := append([]*Player{me}, mates...)
	if len(mates) > 0 {
		names := make([]string, len(mates))
		for k, p := range mates {
			names[k] = p.Name
		}
		g.logf("%s moves, bringing %s.", me.Name, andList(names))
	} else {
		g.logf("%s moves.", me.Name)
	}
	for _, p := range movers {
		p.Pos = to
		g.event("move", Event{"pid": p.ID, "from": from, "to": to})
	}
	for _, p := range movers {
		g.ringCard(p, from, to)
	}
	for _, p := range movers {
		g.autoPlace(p)
	}
	return nil
}

// bring is who comes along when a Challenger moves: everyone standing with
// them (El, 2026-10-09), or the ones the Keeper picked. Off the World Tree
// nobody comes unless picked, since that is where the team gathers.
func (g *Game) bring(me *Player, ids []string) ([]*Player, error) {
	if ids == nil {
		if !me.has(Challenger) || me.Pos == 0 {
			return nil, nil
		}
		var all []*Player
		for _, p := range g.Players {
			if p != me && p.Pos == me.Pos {
				all = append(all, p)
			}
		}
		return all, nil
	}
	if len(ids) > 0 && !me.has(Challenger) {
		return nil, errors.New("only a Challenger brings teammates")
	}
	var mates []*Player
	seen := map[string]bool{}
	for _, id := range ids {
		p := g.player(id)
		if p == nil || p == me || seen[id] {
			return nil, errors.New("choose teammates to bring")
		}
		if p.Pos != me.Pos {
			return nil, fmt.Errorf("%s isn't on your hex", p.Name)
		}
		seen[id] = true
		mates = append(mates, p)
	}
	return mates, nil
}

// andList joins names as "A", "A and B", "A, B and C".
func andList(s []string) string {
	if len(s) <= 1 {
		return strings.Join(s, "")
	}
	return strings.Join(s[:len(s)-1], ", ") + " and " + s[len(s)-1]
}

// ringCard: the first time a player moves inward into Ring 3, 2 or 1, they draw
// that ring's card.
func (g *Game) ringCard(p *Player, from, to int) {
	r := Board[to].Ring
	if r < 1 || r > 3 || from < 0 || r >= Board[from].Ring || p.Reached[r-1] {
		return
	}
	p.Reached[r-1] = true
	g.share("ring", p, g.draw(fmt.Sprintf("ring%d", r)), "", r)
	g.logf("%s reaches Ring %d for the first time.", p.Name, r)
}

// autoPlace: a player standing on the World Tree places everything they carry
// at once. Fruit brings one Heartwood question, however many there are.
func (g *Game) autoPlace(p *Player) {
	if p.Pos != 0 || (p.Fruit == 0 && p.Treasure == "") {
		return
	}
	fruit, tr := p.Fruit, p.Treasure
	p.Placed += fruit
	p.Fruit, p.Treasure = 0, ""
	var what []string
	if fruit > 0 {
		what = append(what, plural(fruit, "fruit", "fruit"))
	}
	if tr != "" {
		g.Placed = append(g.Placed, tr)
		what = append(what, treasureByID(tr).Name)
	}
	g.event("place", Event{"pid": p.ID, "fruit": fruit, "treasure": tr})
	g.logf("%s places %s on the World Tree.", p.Name, strings.Join(what, " and "))
	if tr != "" && len(g.Placed) == len(Treasures) {
		g.logf("%s", HeartComplete)
	}
	if fruit > 0 {
		g.share("heartwood", p, g.draw("heartwood"), "", 0)
	}
}

func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}

func (g *Game) explore(me *Player) error {
	t := g.tile(me.Pos)
	if t == nil || t.Up {
		return errors.New("nothing to explore here")
	}
	cost := 1
	if g.wx(me.Pos) == "fog" {
		cost = 2
	}
	if err := g.spend(cost); err != nil {
		return err
	}
	t.Up = true
	g.event("flip", Event{"hex": me.Pos})
	found := g.turnUp(me, t.Kind) // a sealed find (find.go)
	switch t.Kind {
	case "treasure":
		tr := treasureByID(t.Treasure)
		me.Found = append(me.Found, tr.ID)
		g.logf("%s explores and finds %s %s!", me.Name, tr.Icon, tr.Name)
		g.event("treasure", Event{"hex": me.Pos, "treasure": tr.ID})
		// everyone answers, one at a time, starting with the finder and going round the table
		start := 0
		for k, id := range g.Order {
			if id == me.ID {
				start = k
			}
		}
		for k := range g.Order {
			if p := g.player(g.Order[(start+k)%len(g.Order)]); p != nil {
				g.share("treasure", p, tr.Question, tr.ID, 0)
				s := g.Shares[len(g.Shares)-1]
				s.By, s.Seq, s.Of = me.ID, k+1, len(g.Order)
			}
		}
	case "spring":
		me.Springs++
		g.logf("%s explores and finds a spring 💧.", me.Name)
	default:
		if !found {
			g.logf("%s explores and finds nothing here.", me.Name)
		}
	}
	// An Individualist senses the hexes around: the board shows which face-down
	// ones hold something (a spring or a treasure), not what (El, 2026-10-09).
	if me.has(Individualist) {
		sensed, some := 0, 0
		for _, j := range Adj[me.Pos] {
			if t := g.Tiles[j]; j != 0 && !t.Up && !t.Hint {
				t.Hint = true
				sensed++
				if t.Kind != "empty" {
					some++
				}
			}
		}
		if sensed > 0 {
			g.event("hint", Event{"hex": me.Pos, "pid": me.ID})
			what := "nothing there"
			if some > 0 {
				what = plural(some, "holds something", "hold something")
			}
			g.logf("%s senses the hexes around: %s.", me.Name, what)
		}
	}
	return nil
}

func sowable(t *Tile) bool {
	return t != nil && t.Up && t.Kind != "spring" && t.Treasure == "" && t.Stage == 0 && t.Leaves < 2
}

func (g *Game) sow(me *Player) error {
	if !sowable(g.tile(me.Pos)) {
		return errors.New("sow on an explored, empty hex with nothing growing")
	}
	if err := g.spend(1); err != nil {
		return err
	}
	t := g.Tiles[me.Pos]
	t.Stage = 1
	if me.has(Achiever) {
		t.Stage = 2
	}
	g.event("grow", Event{"hex": me.Pos, "stage": t.Stage})
	if t.Stage == 2 {
		g.logf("%s sows a seed and it springs up as a Sprout.", me.Name)
	} else {
		g.logf("%s sows a seed.", me.Name)
	}
	return nil
}

// reach is the hex a Water or Tend works on: your own, or one next to you for a Helper.
func (g *Game) reach(me *Player, hex int) (int, error) {
	if hex < 0 || hex == me.Pos {
		return me.Pos, nil
	}
	if me.has(Helper) && neighbours(me.Pos, hex) {
		return hex, nil
	}
	return 0, errors.New("stand on it (a Helper can reach a hex next to them)")
}

func (g *Game) water(me *Player, hex int) error {
	j, err := g.reach(me, hex)
	if err != nil {
		return err
	}
	t := g.tile(j)
	if t == nil || (t.Stage != 1 && t.Stage != 2) || t.Leaves >= 2 {
		return errors.New("water a Seeded or Sprout hex that isn't sealed")
	}
	if me.Water < 1 {
		return fmt.Errorf("%s has no water", me.Name)
	}
	if err := g.spend(1); err != nil {
		return err
	}
	me.Water--
	t.Stage++
	g.event("grow", Event{"hex": j, "stage": t.Stage})
	g.logf("%s waters it: %s.", me.Name, StageNames[t.Stage])
	return nil
}

func (g *Game) tend(me *Player, hex int) error {
	j, err := g.reach(me, hex)
	if err != nil {
		return err
	}
	t := g.tile(j)
	if t == nil || t.Stage != 3 || t.Leaves >= 2 {
		return errors.New("tend a Sapling that isn't sealed")
	}
	if err := g.spend(1); err != nil {
		return err
	}
	t.Stage = 4
	g.event("grow", Event{"hex": j, "stage": 4})
	g.logf("%s tends the Sapling: a Big Tree in %s!", me.Name, Values[Board[j].Sector].Name)
	return nil
}

func (g *Game) clear(me *Player, hex int) error {
	if hex < 0 {
		hex = me.Pos
	}
	if hex != me.Pos && !neighbours(me.Pos, hex) {
		return errors.New("clear your hex or one next to you")
	}
	t := g.tile(hex)
	if t == nil || t.Leaves == 0 {
		return errors.New("no dead leaves there")
	}
	cost := 1
	if g.wx(hex) == "rain" && !me.has(Reformer) {
		cost = 2
	}
	if err := g.spend(cost); err != nil {
		return err
	}
	n := 1
	if me.has(Reformer) {
		n = 2
	}
	t.Leaves = max(0, t.Leaves-n)
	g.event("clear", Event{"hex": hex})
	g.logf("%s clears dead leaves.", me.Name)
	return nil
}

func (g *Game) harvest(me *Player) error {
	t := g.tile(me.Pos)
	if t == nil || t.Stage != 4 || t.Leaves >= 2 {
		return errors.New("harvest from a Big Tree that isn't sealed")
	}
	if g.wx(me.Pos) == "rain" {
		return errors.New("no harvest in the rain")
	}
	if t.Harvested {
		return errors.New("this tree was harvested since the last Forest Tide")
	}
	if me.Fruit >= MaxFruit {
		return fmt.Errorf("you can carry %d fruit", MaxFruit)
	}
	if err := g.spend(1); err != nil {
		return err
	}
	t.Harvested = true
	me.Fruit++
	me.Harvested++
	v := Values[Board[me.Pos].Sector]
	g.share("harvest", me, strings.ReplaceAll(HarvestPrompt, "{value}", v.Name), v.Tagline, 0)
	g.event("harvest", Event{"hex": me.Pos, "pid": me.ID})
	g.logf("%s harvests a fruit 🍎.", me.Name)
	return nil
}

func (g *Game) take(me *Player) error {
	t := g.tile(me.Pos)
	if t == nil || !t.Up || t.Treasure == "" {
		return errors.New("no treasure here")
	}
	if me.Treasure != "" {
		return errors.New("you can carry 1 treasure")
	}
	if err := g.spend(1); err != nil {
		return err
	}
	me.Treasure, t.Treasure = t.Treasure, ""
	me.Took = append(me.Took, me.Treasure)
	g.event("take", Event{"hex": me.Pos, "pid": me.ID, "treasure": me.Treasure})
	g.logf("%s takes %s.", me.Name, treasureByID(me.Treasure).Name)
	return nil
}

func (g *Game) drink(me *Player) error {
	t := g.tile(me.Pos)
	if t == nil || !t.Up || t.Kind != "spring" {
		return errors.New("drink at a spring")
	}
	if me.Water >= MaxWater {
		return errors.New("already full")
	}
	if err := g.spend(1); err != nil {
		return err
	}
	me.Water = MaxWater
	g.event("drink", Event{"hex": me.Pos, "pid": me.ID})
	g.logf("%s fills up at the spring 💧.", me.Name)
	return nil
}

// canPass: on the same hex, or to a teammate on a neighbouring hex when the
// two are in a Peacemaker's chain: teammates linked hex by hex, each on or next
// to the next one's hex, with a Peacemaker among them (El, 2026-10-09).
func (g *Game) canPass(a, b *Player) bool {
	if a == b || a.Pos < 0 || b.Pos < 0 {
		return false
	}
	if a.Pos == b.Pos {
		return true
	}
	return neighbours(a.Pos, b.Pos) && g.inChain(a)
}

// inChain says whether p is linked to a Peacemaker through teammates standing
// on the same or neighbouring hexes (a Peacemaker is in their own chain).
func (g *Game) inChain(p *Player) bool {
	seen := map[string]bool{p.ID: true}
	todo := []*Player{p}
	for len(todo) > 0 {
		a := todo[0]
		todo = todo[1:]
		if a.has(Peacemaker) {
			return true
		}
		for _, b := range g.Players {
			if !seen[b.ID] && b.Pos >= 0 && (b.Pos == a.Pos || neighbours(a.Pos, b.Pos)) {
				seen[b.ID] = true
				todo = append(todo, b)
			}
		}
	}
	return false
}

// pass is free: on your own turn, water, fruit or a treasure to a teammate you can reach.
func (g *Game) pass(a, b *Player, item string) error {
	if g.Phase != PhaseTurn || g.Turn == nil {
		return errors.New("pass things during the game")
	}
	if g.Turn.Player != a.ID {
		return fmt.Errorf("only %s can pass now, on their turn", g.current().Name)
	}
	if b == nil || b == a {
		return errors.New("choose a teammate to pass to")
	}
	if !g.canPass(a, b) {
		return errors.New("pass on the same hex, or to a teammate next to you in a Peacemaker's chain")
	}
	switch item {
	case "water":
		if a.Water < 1 {
			return fmt.Errorf("%s has no water to pass", a.Name)
		}
		if b.Water >= MaxWater {
			return fmt.Errorf("%s is full", b.Name)
		}
		a.Water--
		b.Water++
	case "fruit":
		if a.Fruit < 1 {
			return fmt.Errorf("%s has no fruit to pass", a.Name)
		}
		if b.Fruit >= MaxFruit {
			return fmt.Errorf("%s carries %d fruit already", b.Name, MaxFruit)
		}
		a.Fruit--
		b.Fruit++
	case "treasure":
		if a.Treasure == "" {
			return fmt.Errorf("%s has no treasure to pass", a.Name)
		}
		if b.Treasure != "" {
			return fmt.Errorf("%s carries a treasure already", b.Name)
		}
		b.Treasure, a.Treasure = a.Treasure, ""
	default:
		return errors.New("pass water, fruit or a treasure")
	}
	g.event("pass", Event{"from": a.ID, "to": b.ID, "item": item})
	g.logf("%s passes %s to %s.", a.Name, item, b.Name)
	g.autoPlace(b)
	return nil
}

// investigate: once a round, on their own turn, an Investigator sees one
// sector's next weather on their phone.
func (g *Game) investigate(me *Player, sector int) error {
	if !me.has(Investigator) {
		return errors.New("only an Investigator looks ahead")
	}
	if g.Phase != PhaseTurn || g.Turn == nil || g.Turn.Player != me.ID {
		return errors.New("look during your own turn")
	}
	if g.Turn.Investigated {
		return errors.New("you've looked once this round")
	}
	if sector < 0 || sector > 5 {
		return errors.New("choose a sector")
	}
	g.Turn.Investigated = true
	me.Peeks = append(me.Peeks, Peek{Type: "weather", Hex: -1, Sector: sector, Weather: g.NextWeather[sector],
		Round: g.Round, Tide: g.Tide})
	return nil
}

func (g *Game) endTurn() {
	g.TurnIdx++
	if g.TurnIdx >= len(g.Order) {
		g.forestTide()
		if g.Phase != PhaseTurn {
			return
		}
		g.TurnIdx = 0
		g.Round++
		g.logf("Round %d.", g.Round)
	}
	g.newTurn()
}

// forestTide runs after everyone's turn: new weather, rain grows plants, sun
// dries walkers, and the Forest Breath blows dead leaves toward the centre.
func (g *Game) forestTide() {
	g.Tide++
	g.Weather = g.NextWeather
	for s := range 6 {
		g.NextWeather[s] = drawWeather()
	}
	growth, dry, leaves, sealed := []int{}, []string{}, []Drift{}, []int{}
	for i, t := range g.Tiles {
		if t == nil {
			continue
		}
		t.Harvested = false
		if g.wx(i) == "rain" && (t.Stage == 1 || t.Stage == 2) && t.Leaves < 2 {
			t.Stage++
			growth = append(growth, i)
		}
	}
	for _, p := range g.Players {
		if g.wx(p.Pos) == "sun" && g.Tiles[p.Pos].Stage != 4 && p.Water > 0 {
			p.Water--
			dry = append(dry, p.ID)
		}
	}
	// A plant is safe while someone stands on it (El, 2026-10-08); a Loyalist
	// also keeps the 6 hexes around them safe.
	safe := map[int]bool{}
	for _, p := range g.Players {
		if p.Pos > 0 {
			safe[p.Pos] = true
		}
		if p.has(Loyalist) && p.Pos >= 0 {
			safe[p.Pos] = true
			for _, j := range Adj[p.Pos] {
				safe[j] = true
			}
		}
	}
	for _, b := range g.Breath {
		t := g.Tiles[b.To]
		if t.Leaves >= 2 {
			continue
		}
		t.Leaves++
		if t.Stage >= 1 && t.Stage <= 3 && !safe[b.To] {
			t.Stage--
		}
		leaves = append(leaves, b)
		if t.Leaves >= 2 {
			sealed = append(sealed, b.To)
		}
	}
	g.Breath = g.rollBreath()
	g.event("tide", Event{"weather": g.Weather, "growth": growth, "dry": dry, "leaves": leaves, "sealed": sealed})
	icons := map[string]string{"sun": "☀️", "rain": "🌧️", "fog": "🌫️"}
	var ws []string
	for _, w := range g.Weather {
		ws = append(ws, icons[w])
	}
	g.logf("🌬️ Forest Tide %d: %s · %d drifts of dead leaves.", g.Tide, strings.Join(ws, " "), len(leaves))
	g.check()
}

// Goals is how far the team is toward waking the forest.
type Goals struct {
	Trees         [6]bool  `json:"trees"` // a Big Tree in each value
	PlacedPlayers int      `json:"placedPlayers"`
	Players       int      `json:"players"`
	Treasures     []string `json:"treasures"` // placed on the World Tree
	OnTree        int      `json:"onTree"`
}

func (g *Game) goals() Goals {
	o := Goals{Players: len(g.Players), Treasures: append([]string{}, g.Placed...)}
	for i, t := range g.Tiles {
		if t != nil && t.Stage == 4 {
			o.Trees[Board[i].Sector] = true
		}
	}
	for _, p := range g.Players {
		if p.Placed > 0 {
			o.PlacedPlayers++
		}
		if p.Pos == 0 {
			o.OnTree++
		}
	}
	return o
}

func (o Goals) won() bool {
	for _, t := range o.Trees {
		if !t {
			return false
		}
	}
	return o.PlacedPlayers == o.Players && len(o.Treasures) >= len(Treasures) && o.OnTree == o.Players
}

// check ends the game the moment the forest wakes, or everyone is dry.
func (g *Game) check() {
	if g.Phase != PhaseTurn || g.Result != "" {
		return
	}
	if g.goals().won() {
		g.end("won")
		g.logf("🌳 The forest wakes!")
		g.event("wake", nil)
		g.toGuess()
		return
	}
	for _, p := range g.Players {
		if p.Water > 0 {
			return
		}
	}
	g.end("lost")
	g.logf("Everyone is out of water. The forest sleeps.")
	g.event("dry", nil)
	g.toGuess()
}

// end sets the result and notes when, so the photos can be deleted a while later.
func (g *Game) end(result string) {
	g.Result = result
	if g.EndedAt == 0 {
		g.EndedAt = time.Now().Unix()
	}
}

func (g *Game) toGuess() {
	g.Phase = PhaseGuess
	g.Turn = nil
	g.Undo = nil
	g.TimerEnd, g.TimerLabel = 0, ""
	g.logf("Everyone: guess who was your Secret Owl.")
}

// ---- recognition ----

// Recognition is what the end screen shows for each player. No points, no
// winner among players: the team won or lost together.
type Recognition struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Color        string   `json:"color"`
	Trust        int      `json:"trust"`        // trust acorns received
	Givers       int      `json:"givers"`       // from how many people
	OwlName      string   `json:"owlName"`      // who was their Secret Owl
	GuessedRight bool     `json:"guessedRight"` // they spotted their Owl
	OwlHidden    bool     `json:"owlHidden"`    // as an Owl, they stayed hidden from the one they watched
	TargetName   string   `json:"targetName"`   // the one they watched
	Value        int      `json:"value"`        // the value they stood for (-1: never entered)
	Found        []string `json:"found"`        // treasures they turned up exploring
	Took         []string `json:"took"`         // treasures they picked up
	Harvested    int      `json:"harvested"`    // fruit they harvested
	Springs      int      `json:"springs"`      // springs they found exploring
}

func (g *Game) owlOf(id string) *Player {
	for _, p := range g.Players {
		if p.Target == id {
			return p
		}
	}
	return nil
}

func (g *Game) Recognition() []Recognition {
	out := []Recognition{}
	for _, p := range g.Players {
		r := Recognition{ID: p.ID, Name: p.Name, Color: p.Color, Value: p.Value, Found: append([]string{}, p.Found...),
			Took: append([]string{}, p.Took...), Harvested: p.Harvested, Springs: p.Springs}
		for _, c := range p.Pot {
			if c > 0 {
				r.Trust += c
				r.Givers++
			}
		}
		if owl := g.owlOf(p.ID); owl != nil {
			r.OwlName = owl.Name
			r.GuessedRight = p.Guess == owl.ID
		}
		if t := g.player(p.Target); t != nil {
			r.TargetName = t.Name
			r.OwlHidden = t.Guess != p.ID
		}
		out = append(out, r)
	}
	return out
}

// ---- actions ----

func (g *Game) Apply(a Action) error {
	defer g.syncShareTimer()
	isHost := a.Host != "" && a.Host == g.HostSecret
	if isHost && a.As == "" && keeperActs[a.Type] && (g.Phase == PhaseEnter || g.Phase == PhaseTurn) {
		// Without "as", the Keeper acts for whoever's turn it is.
		if cur := g.current(); cur != nil {
			a.As = cur.ID
		}
	}
	if isHost && a.As != "" {
		// Players say it aloud and the Keeper taps it on the big screen. Private
		// choices (trust, the Investigator's look, the Secret Owl guess) stay on
		// their phones.
		if !keeperActs[a.Type] {
			return errors.New("players do that on their own phone")
		}
		p := g.player(a.As)
		if p == nil {
			return errors.New("no such player")
		}
		return g.keeperTap(p, a)
	}
	if isHost {
		return g.hostAction(a)
	}
	me := g.player(a.Pid)
	if me == nil || me.Secret != a.Secret {
		return errors.New("not recognised: rejoin the game")
	}
	return g.playerAction(me, a)
}

// keeperActs lists the player actions the Keeper may take for a player.
var keeperActs = map[string]bool{
	"enter": true, "move": true, "explore": true, "sow": true, "water": true, "tend": true, "clear": true,
	"harvest": true, "take": true, "drink": true, "pass": true, "endTurn": true,
}

func (g *Game) hostAction(a Action) error {
	switch a.Type {
	case "start":
		return g.start()
	case "slide":
		return g.slide(a.N)
	case "begin":
		return g.begin()
	case "assignPowers":
		return g.assignPowers(a.Target, a.Types)
	case "randomTypes":
		return g.randomTypes()
	case "kick":
		if g.Phase != PhaseLobby {
			return errors.New("only in the lobby")
		}
		for i, p := range g.Players {
			if p.ID == a.Target {
				g.Players = append(g.Players[:i], g.Players[i+1:]...)
				g.logf("%s left", p.Name)
				return nil
			}
		}
		return errors.New("no such player")
	case "doneShare":
		return g.doneShare()
	case "another":
		return g.another(nil)
	case "startChain":
		if g.Phase != PhaseGuess {
			return errors.New("not now")
		}
		if len(g.Shares) > 0 {
			return errors.New("finish the share first")
		}
		g.Phase = PhaseChain
		g.ChainIdx = 0
		g.setTimer(60, "Tribute")
		g.logf("The tribute chain: each Secret Owl honours the one they watched.")
	case "nextTribute":
		if g.Phase != PhaseChain {
			return errors.New("not now")
		}
		g.ChainIdx++
		if g.ChainIdx >= len(g.Chain) {
			g.Phase = PhaseEnd
			g.TimerEnd, g.TimerLabel = 0, ""
			if g.Result == "won" {
				g.logf("The forest is awake. Thank you, all of you.")
			} else {
				g.logf("The forest sleeps, for now. Thank you, all of you.")
			}
			return nil
		}
		g.setTimer(60, "Tribute")
	case "endGame":
		// The Keeper closes the game early (time's up): straight to the finale.
		if g.Phase != PhaseTurn {
			return errors.New("the game can be closed early only during play")
		}
		if len(g.Shares) > 0 {
			return errors.New("finish the share first")
		}
		g.end("ended")
		g.logf("The Keeper closed the game early.")
		g.toGuess()
	case "removePhoto":
		// The Keeper takes a player's photo off the board (the server deletes it).
		p := g.player(a.Target)
		if p == nil {
			return errors.New("no such player")
		}
		if p.Photo == 0 {
			return fmt.Errorf("%s has no photo", p.Name)
		}
		p.Photo = 0
	case "timer":
		g.setTimer(a.N, "Timer")
	case "undo":
		return g.undo()
	case "foundCheer":
		return g.foundStage("cheer")
	case "foundClose":
		return g.foundStage("done")
	default:
		if ok, err := g.recordAction(a); ok { // record, recordHere
			return err
		}
		return errors.New("unknown host action")
	}
	return nil
}

func (g *Game) playerAction(me *Player, a Action) error {
	switch a.Type {
	case "trust":
		return g.giveTrust(me)
	case "another":
		return g.another(me)
	case "guess":
		if g.Phase != PhaseGuess {
			return errors.New("guess at the end of the game")
		}
		if a.Target == me.ID || g.player(a.Target) == nil {
			return errors.New("guess someone else")
		}
		me.Guess = a.Target
		return nil
	case "investigate", "enter", "pass", "move", "explore", "sow", "water", "tend", "clear",
		"harvest", "take", "drink", "endTurn":
	default:
		return errors.New("unknown action")
	}

	// Everything below is play: it waits while someone is sharing.
	if len(g.Shares) > 0 {
		return errors.New("wait for the share to finish (the Keeper taps Done)")
	}
	switch a.Type {
	case "investigate":
		return g.investigate(me, a.N)
	case "enter":
		return g.enter(me, a.N, a.Hex)
	case "pass":
		err := g.pass(me, g.player(a.Target), a.Text)
		if err == nil {
			g.check()
		}
		return err
	}

	// The rest is the current player's own turn.
	if g.Phase != PhaseTurn || g.Turn == nil {
		return errors.New("not now")
	}
	if g.Turn.Player != me.ID {
		return fmt.Errorf("it's %s's turn", g.current().Name)
	}
	var err error
	switch a.Type {
	case "move":
		err = g.move(me, a.Hex, a.Bring)
	case "explore":
		err = g.explore(me)
	case "sow":
		err = g.sow(me)
	case "water":
		err = g.water(me, a.Hex)
	case "tend":
		err = g.tend(me, a.Hex)
	case "clear":
		err = g.clear(me, a.Hex)
	case "harvest":
		err = g.harvest(me)
	case "take":
		err = g.take(me)
	case "drink":
		err = g.drink(me)
	case "endTurn":
		g.endTurn()
		return nil
	}
	if err == nil {
		g.check()
	}
	return err
}
