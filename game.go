package main

import (
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"time"
)

const (
	PhaseLobby  = "lobby"
	PhasePlant  = "plant"
	PhaseTurn   = "turn"
	PhaseDusk   = "dusk"
	PhaseGuess  = "guess"
	PhaseChain  = "chain"
	PhaseScores = "scores"
)

var SeasonNames = [4]string{"", "Planting the Seed", "The Sprout", "Growing into a Tree"}

type Hex struct {
	Q      int `json:"q"`
	R      int `json:"r"`
	Ring   int `json:"ring"`
	Region int `json:"region"` // -1 for the Heartwood
}

type Token struct {
	Kind    string `json:"kind"`
	Flipped bool   `json:"flipped"`
}

type KeptCard struct {
	Region int    `json:"region"`
	Tier   int    `json:"tier"`
	Text   string `json:"text"`
}

type Player struct {
	ID        string         `json:"id"`
	Secret    string         `json:"secret"`
	Name      string         `json:"name"`
	Color     string         `json:"color"`
	Types     []int          `json:"types"`
	Used      map[int]bool   `json:"used"`
	Pos       int            `json:"pos"`
	Bonus     int            `json:"bonus"` // bonus points (✨)
	TrustLeft int            `json:"trustLeft"`
	Squirrels int            `json:"squirrels"`
	Pot       map[string]int `json:"pot"`
	Cards     []KeptCard     `json:"cards"`
	Allies    []string       `json:"allies"`
	Partners  []string       `json:"partners"`
	Target    string         `json:"target"`
	Guess     string         `json:"guess"`
	Confirmed int            `json:"confirmed"`
	Peek      map[int]string `json:"peek"`
}

type TurnState struct {
	Player     string         `json:"player"`
	Step       string         `json:"step"` // move, share, trust
	Discovered bool           `json:"discovered"`
	StepsLeft  int            `json:"stepsLeft"` // up to 3 a turn, more with Momentum
	Teleport   bool           `json:"teleport"`
	CanClaim   bool           `json:"canClaim"`
	Region     int            `json:"region"`
	Tier       int            `json:"tier"`
	Card       int            `json:"card"`
	Prompt     string         `json:"prompt"`
	Choices    []int          `json:"choices"`
	Redrawn    bool           `json:"redrawn"`
	Started    bool           `json:"started"`
	Bonus      int            `json:"bonus"`
	ScoreTier  int            `json:"scoreTier"` // set by Deep Water: the tier the share scores at
	Invited    string         `json:"invited"`   // Open Hands: answers the same question too
	Campfire   string         `json:"campfire"`  // Adventure: the Campfire question called this turn
	Event      string         `json:"event"`
	EventText  string         `json:"eventText"`
	Trusted    map[string]int `json:"trusted"`
	FollowUps  []string       `json:"followUps"`
}

type DuskChoice struct {
	Partner string `json:"partner"`
	Ally    bool   `json:"ally"`
}

type Pair struct {
	A    string `json:"a"`
	B    string `json:"b"`
	C    string `json:"c,omitempty"`
	Ally bool   `json:"ally"`
	Bond string `json:"bond"`
	Note string `json:"note,omitempty"` // why a requested alliance didn't form
}

type DuskState struct {
	Choices  map[string]DuskChoice `json:"choices"`
	Common   map[string]bool       `json:"common"`
	Pairs    []Pair                `json:"pairs"`
	Resolved bool                  `json:"resolved"`
}

type Game struct {
	Code       string               `json:"code"`
	HostSecret string               `json:"hostSecret"`
	Phase      string               `json:"phase"`
	Season     int                  `json:"season"`
	ForestGoal bool                 `json:"forestGoal"`
	Players    []*Player            `json:"players"`
	Order      []string             `json:"order"`
	TurnIdx    int                  `json:"turnIdx"`
	StartIdx   int                  `json:"startIdx"`
	TurnsTaken int                  `json:"turnsTaken"`
	Hexes      []Hex                `json:"hexes"`
	Tokens     map[int]*Token       `json:"tokens"`
	Trees      map[int]int          `json:"trees"`
	Decks      map[int][]int        `json:"decks"` // 0..5 regions, 6 Heartwood
	Turn       *TurnState           `json:"turn"`
	Dusk       *DuskState           `json:"dusk"`
	Chain      []string             `json:"chain"`
	ChainIdx   int                  `json:"chainIdx"`
	Oak        map[int]bool         `json:"oak"`
	Log        []string             `json:"log"`
	TimerEnd   int64                `json:"timerEnd"`
	TimerLabel string               `json:"timerLabel"`
	Version    int                  `json:"version"`
	Rejoin     map[string]RejoinPin `json:"rejoin"`
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
	Hex    int    `json:"hex"`
	Target string `json:"target"`
	Power  int    `json:"power"`
	Ally   bool   `json:"ally"`
	N      int    `json:"n"`
	Card   int    `json:"card"`
	Forest bool   `json:"forest"`
	Types  []int  `json:"types"`
	As     string `json:"as"` // the player the Keeper acts for
}

func buildHexes() []Hex {
	hs := []Hex{{0, 0, 0, -1}}
	dirs := [6][2]int{{1, 0}, {1, -1}, {0, -1}, {-1, 0}, {-1, 1}, {0, 1}}
	for k := 1; k <= 3; k++ {
		q, r := dirs[4][0]*k, dirs[4][1]*k
		for side := 0; side < 6; side++ {
			for s := 0; s < k; s++ {
				hs = append(hs, Hex{q, r, k, side})
				q += dirs[side][0]
				r += dirs[side][1]
			}
		}
	}
	return hs
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func (g *Game) dist(a, b int) int {
	ha, hb := g.Hexes[a], g.Hexes[b]
	dq, dr := ha.Q-hb.Q, ha.R-hb.R
	return (abs(dq) + abs(dr) + abs(dq+dr)) / 2
}

func NewGame(code, hostSecret string) *Game {
	g := &Game{Code: code, HostSecret: hostSecret, Phase: PhaseLobby, Hexes: buildHexes(),
		Tokens: map[int]*Token{}, Trees: map[int]int{}, Decks: map[int][]int{}, Oak: map[int]bool{}}
	var kinds []string
	for k, n := range TokenMix {
		for i := 0; i < n; i++ {
			kinds = append(kinds, k)
		}
	}
	sort.Strings(kinds)
	// Spaces beyond the mix are quiet clearings with nothing to find.
	for len(kinds) < len(g.Hexes)-1 {
		kinds = append(kinds, "")
	}
	rand.Shuffle(len(kinds), func(i, j int) { kinds[i], kinds[j] = kinds[j], kinds[i] })
	for i := 1; i < len(g.Hexes); i++ {
		if kinds[i-1] != "" {
			g.Tokens[i] = &Token{Kind: kinds[i-1]}
		}
	}
	return g
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
	g.Log = append(g.Log, fmt.Sprintf(format, a...))
	if len(g.Log) > 200 {
		g.Log = g.Log[len(g.Log)-200:]
	}
}

func (g *Game) setTimer(secs int, label string) {
	g.TimerEnd = time.Now().Add(time.Duration(secs) * time.Second).UnixMilli()
	g.TimerLabel = label
}

func (g *Game) draw(deck int) int {
	if len(g.Decks[deck]) == 0 {
		n := len(HeartwoodCards)
		if deck < 6 {
			n = len(RegionCards[deck])
		}
		g.Decks[deck] = rand.Perm(n)
	}
	c := g.Decks[deck][0]
	g.Decks[deck] = g.Decks[deck][1:]
	return c
}

func (g *Game) ringOpen(ring int) bool {
	switch g.Season {
	case 1:
		return ring == 3
	case 2:
		return ring >= 2
	default:
		return true
	}
}

func hasAlly(p *Player, id string) bool {
	for _, a := range p.Allies {
		if a == id {
			return true
		}
	}
	return false
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

// ---- lobby ----

func (g *Game) Join(name, color string) (*Player, error) {
	if g.Phase != PhaseLobby {
		return nil, errors.New("the game has already started")
	}
	if len(g.Players) >= 10 {
		return nil, errors.New("the game is full (10 players)")
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
	p := &Player{ID: randID(6), Secret: randID(16), Name: name, Color: color,
		Used: map[int]bool{}, Pos: -1, Pot: map[string]int{}, Peek: map[int]string{}}
	g.Players = append(g.Players, p)
	g.logf("%s joined", name)
	return p, nil
}

// assignPowers is the Keeper giving a player 1 to 3 Enneagram types in the
// lobby; each type's power can be used once. An empty list clears them.
func (g *Game) assignPowers(pid string, types []int) error {
	if g.Phase != PhaseLobby {
		return errors.New("powers are assigned in the lobby")
	}
	p := g.player(pid)
	if p == nil {
		return errors.New("no such player")
	}
	if len(types) > 3 {
		return errors.New("up to 3 powers per player")
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

func (g *Game) start(forest bool) error {
	if g.Phase != PhaseLobby {
		return errors.New("already started")
	}
	if len(g.Players) < 2 {
		return errors.New("need at least 2 players")
	}
	for _, p := range g.Players {
		if len(p.Types) == 0 {
			return fmt.Errorf("assign at least one Enneagram power to %s", p.Name)
		}
	}
	g.ForestGoal = forest
	for _, p := range g.Players {
		g.Order = append(g.Order, p.ID)
		p.TrustLeft = 10
	}
	// Secret Owl: one loop through a shuffled order. Each player is the Secret
	// Owl of their Target, watching them quietly all game.
	perm := rand.Perm(len(g.Players))
	for i, pi := range perm {
		g.Players[pi].Target = g.Players[perm[(i+1)%len(perm)]].ID
		g.Chain = append(g.Chain, g.Players[pi].ID)
	}
	g.Phase = PhasePlant
	g.TurnIdx, g.TurnsTaken = 0, 0
	g.logf("The expedition begins. Plant your seed: choose a Seedlands space in the value you identify with most, and say why.")
	return nil
}

// ---- turns ----

func (g *Game) current() *Player {
	return g.player(g.Order[g.TurnIdx])
}

func (g *Game) newTurn() {
	g.Turn = &TurnState{Player: g.Order[g.TurnIdx], Step: "move", Trusted: map[string]int{}, Region: -1, StepsLeft: 3}
	g.TimerEnd, g.TimerLabel = 0, ""
	g.current().Peek = map[int]string{}
}

// canEnter says whether p may stand on space to: its ring must be open, and
// no one reaches the Heartwood without an ally.
func (g *Game) canEnter(p *Player, to int) error {
	if to < 0 || to >= len(g.Hexes) {
		return errors.New("no such space")
	}
	h := g.Hexes[to]
	if !g.ringOpen(h.Ring) {
		return errors.New("that ring opens in a later season")
	}
	if h.Ring == 0 && len(p.Allies) == 0 {
		return errors.New("no one reaches the Heartwood alone: you need an alliance")
	}
	return nil
}

func (g *Game) resolveToken(p *Player, hex int) {
	t := g.Tokens[hex]
	if t == nil || t.Flipped {
		return
	}
	t.Flipped = true
	tu := g.Turn
	tu.Event = t.Kind
	switch t.Kind {
	case "mushroom":
		p.Bonus += 2
		tu.EventText = "Mushroom patch: +2 ✨ bonus points"
	case "squirrel":
		p.Squirrels++
		tu.EventText = "Squirrel: store it. Right after someone else's share, play it to ask them a follow-up question."
	case "campfire":
		tu.EventText = "Campfire! Everyone answers in one sentence: " + CampfireCards[rand.Intn(len(CampfireCards))]
	case "path":
		tu.Teleport = true
		tu.EventText = "Hidden path: move free to any open space in this ring, or stay."
	}
	g.logf("%s found: %s", p.Name, tu.EventText)
}

func (g *Game) dealCard(p *Player) {
	tu := g.Turn
	h := g.Hexes[p.Pos]
	tu.Region = h.Region
	if h.Ring == 0 {
		tu.Tier = 4
		tu.Card = g.draw(6)
	} else {
		tu.Tier = 4 - h.Ring
		tu.Card = g.draw(h.Region)
	}
	g.setPrompt()
}

func (g *Game) setPrompt() {
	tu := g.Turn
	if tu.Tier == 4 {
		tu.Prompt = HeartwoodCards[tu.Card]
	} else {
		tu.Prompt = RegionCards[tu.Region][tu.Card][tu.Tier-1]
	}
}

func (g *Game) toShare(p *Player) {
	g.Turn.Step = "share"
	g.Turn.Teleport = false
	g.Turn.CanClaim = false
	g.dealCard(p)
}

func shareSecs(tier int) int {
	switch tier {
	case 1:
		return 45
	case 2:
		return 60
	}
	return 90
}

func (g *Game) finishTurn() {
	p := g.current()
	tu := g.Turn
	if tu.Prompt != "" {
		scored := tu.Tier
		if tu.ScoreTier > 0 && tu.ScoreTier < scored {
			scored = tu.ScoreTier
		}
		p.Cards = append(p.Cards, KeptCard{tu.Region, scored, tu.Prompt})
		g.Trees[p.Pos]++
		if o := g.player(tu.Invited); o != nil {
			// Open Hands: the invited player answered the same question and keeps it too.
			o.Cards = append(o.Cards, KeptCard{tu.Region, scored, tu.Prompt})
			g.Trees[p.Pos]++
		}
		if tu.Tier == 3 {
			g.Oak[tu.Region] = true
		}
		n := 0
		for _, c := range tu.Trusted {
			n += c
		}
		g.logf("%s shared (%s) and received %d trust", p.Name, tierName(tu.Tier), n)
	}
	p.Bonus += tu.Bonus
	g.TurnsTaken++
	if g.TurnsTaken >= len(g.Order) {
		g.enterDusk()
		return
	}
	g.TurnIdx = (g.TurnIdx + 1) % len(g.Order)
	g.newTurn()
}

func tierName(t int) string {
	return [5]string{"", "Seed", "Sapling", "Oak", "Heartwood"}[t]
}

// ---- dusk ----

func (g *Game) enterDusk() {
	g.Phase = PhaseDusk
	g.Turn = nil
	g.TimerEnd, g.TimerLabel = 0, ""
	g.Dusk = &DuskState{Choices: map[string]DuskChoice{}, Common: map[string]bool{}}
	g.logf("Dusk falls on season %d. Choose a partner you haven't talked with yet.", g.Season)
}

func (g *Game) adjacentOrSame(a, b *Player) bool {
	return g.dist(a.Pos, b.Pos) <= 1
}

func (g *Game) resolveDusk() {
	d := g.Dusk
	paired := map[string]bool{}
	var pairs []Pair
	ids := append([]string(nil), g.Order...)
	for _, id := range ids {
		if paired[id] {
			continue
		}
		c, ok := d.Choices[id]
		if !ok || c.Partner == "" || paired[c.Partner] {
			continue
		}
		oc, ok := d.Choices[c.Partner]
		if !ok || oc.Partner != id {
			continue
		}
		a, b := g.player(id), g.player(c.Partner)
		if a == nil || b == nil || contains(a.Partners, b.ID) {
			continue
		}
		pr := Pair{A: a.ID, B: b.ID}
		if c.Ally || oc.Ally {
			// Say why an alliance someone asked for didn't happen.
			common := d.Common[a.ID] || d.Common[b.ID]
			switch {
			case !(c.Ally && oc.Ally):
				pr.Note = "Only one of you asked for an alliance."
			case hasAlly(a, b.ID):
				pr.Note = "You're already allies."
			case len(a.Allies) >= 3 || len(b.Allies) >= 3:
				pr.Note = "One of you already has 3 alliances."
			case !common && !g.adjacentOrSame(a, b):
				pr.Note = "No alliance: you weren't on the same or neighbouring spaces."
			default:
				a.Allies = append(a.Allies, b.ID)
				b.Allies = append(b.Allies, a.ID)
				pr.Ally = true
			}
		}
		paired[a.ID], paired[b.ID] = true, true
		pairs = append(pairs, pr)
	}
	// Everyone else gets a Fireside chat, avoiding repeat partners where possible.
	var rest []string
	for _, id := range ids {
		if !paired[id] {
			rest = append(rest, id)
		}
	}
	rand.Shuffle(len(rest), func(i, j int) { rest[i], rest[j] = rest[j], rest[i] })
	for len(rest) >= 2 {
		a := g.player(rest[0])
		j := 1
		for k := 1; k < len(rest); k++ {
			if !contains(a.Partners, rest[k]) {
				j = k
				break
			}
		}
		pairs = append(pairs, Pair{A: rest[0], B: rest[j]})
		rest = append(rest[1:j], rest[j+1:]...)
	}
	if len(rest) == 1 && len(pairs) > 0 {
		pairs[len(pairs)-1].C = rest[0]
	}
	bonds := BondCards[g.Season-1]
	for i := range pairs {
		pr := &pairs[i]
		pr.Bond = bonds[rand.Intn(len(bonds))]
		members := []string{pr.A, pr.B}
		if pr.C != "" {
			members = append(members, pr.C)
		}
		for _, m := range members {
			p := g.player(m)
			for _, o := range members {
				if o != m && !contains(p.Partners, o) {
					p.Partners = append(p.Partners, o)
				}
			}
		}
		if pr.Ally {
			g.logf("Alliance: %s & %s", g.player(pr.A).Name, g.player(pr.B).Name)
		}
	}
	d.Pairs = pairs
	d.Resolved = true
	g.setTimer(240, "Dusk conversations")
}

func (g *Game) nextSeason() {
	g.Dusk = nil
	if g.Season >= 3 {
		g.Phase = PhaseGuess
		g.TimerEnd, g.TimerLabel = 0, ""
		g.logf("The Vision of a Forest. Everyone: guess who was your Secret Owl.")
		return
	}
	g.Season++
	g.Phase = PhaseTurn
	g.StartIdx = (g.StartIdx + 1) % len(g.Order)
	g.TurnIdx, g.TurnsTaken = g.StartIdx, 0
	g.newTurn()
	g.logf("Season %d: %s", g.Season, SeasonNames[g.Season])
}

// ---- scoring ----

type Score struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Color     string `json:"color"`
	Trust     int    `json:"trust"`
	Givers    int    `json:"givers"`
	Growth    int    `json:"growth"`
	Alliances int    `json:"alliances"`
	Secret    int    `json:"secret"`
	Total     int    `json:"total"`
	OwlName   string `json:"owlName"`
	Winner    bool   `json:"winner"`
}

func (g *Game) owlOf(id string) *Player {
	for _, p := range g.Players {
		if p.Target == id {
			return p
		}
	}
	return nil
}

func (g *Game) Scores() ([]Score, bool) {
	var out []Score
	for _, p := range g.Players {
		s := Score{ID: p.ID, Name: p.Name, Color: p.Color}
		for _, c := range p.Pot {
			if c > 0 {
				s.Trust += min(c, 2) + 1
				s.Givers++
			}
		}
		regions := map[int]bool{}
		for _, c := range p.Cards {
			s.Growth += c.Tier
			if c.Region >= 0 {
				regions[c.Region] = true
			}
		}
		s.Growth += len(regions) + p.Bonus
		s.Alliances = 3 * len(p.Allies)
		s.Secret = p.Confirmed
		if t := g.player(p.Target); t != nil && t.Guess != p.ID {
			s.Secret += 2
		}
		if owl := g.owlOf(p.ID); owl != nil {
			s.OwlName = owl.Name
			if p.Guess == owl.ID {
				s.Secret += 2
			}
		}
		s.Total = s.Trust + s.Growth + s.Alliances + s.Secret
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Total != out[j].Total {
			return out[i].Total > out[j].Total
		}
		return out[i].Trust > out[j].Trust
	})
	forestStands := len(g.Oak) == 6
	if len(out) > 0 && (!g.ForestGoal || forestStands) {
		for i := range out {
			if out[i].Total == out[0].Total && out[i].Trust == out[0].Trust {
				out[i].Winner = true // a true tie shares the win
			}
		}
	}
	return out, forestStands
}

// ---- actions ----

func (g *Game) Apply(a Action) error {
	isHost := a.Host != "" && a.Host == g.HostSecret
	if isHost && a.As != "" {
		// Players say it aloud and the Keeper taps it on the big screen. Private
		// choices (trust, Dusk partner, Secret Owl guess) stay on their phones.
		if !keeperActs[a.Type] {
			return errors.New("players do that on their own phone")
		}
		p := g.player(a.As)
		if p == nil {
			return errors.New("no such player")
		}
		return g.playerAction(p, a)
	}
	var me *Player
	if !isHost {
		me = g.player(a.Pid)
		if me == nil || me.Secret != a.Secret {
			return errors.New("not recognised: rejoin the game")
		}
	}
	if isHost {
		return g.hostAction(a)
	}
	return g.playerAction(me, a)
}

// keeperActs lists the player actions the Keeper may take for a player.
var keeperActs = map[string]bool{
	"plant": true, "step": true, "endMove": true, "claim": true, "redraw": true, "lighter": true,
	"pickChoice": true, "pass": true, "startShare": true, "doneShare": true, "endTrust": true,
	"power": true, "squirrel": true, "confirm": true,
}

func (g *Game) hostAction(a Action) error {
	switch a.Type {
	case "start":
		return g.start(a.Forest)
	case "assignPowers":
		return g.assignPowers(a.Target, a.Types)
	case "kick":
		if g.Phase != PhaseLobby {
			return errors.New("only in the lobby")
		}
		for i, p := range g.Players {
			if p.ID == a.Target {
				g.Players = append(g.Players[:i], g.Players[i+1:]...)
				return nil
			}
		}
		return errors.New("no such player")
	case "endTrust":
		if g.Phase != PhaseTurn || g.Turn == nil || g.Turn.Step != "trust" {
			return errors.New("not collecting trust now")
		}
		g.finishTurn()
	case "skipTurn":
		if g.Phase == PhasePlant {
			return errors.New("players must plant their seed")
		}
		if g.Phase != PhaseTurn {
			return errors.New("no turn to skip")
		}
		g.logf("%s's turn was skipped", g.current().Name)
		g.Turn.Prompt = ""
		g.finishTurn()
	case "resolveDusk":
		if g.Phase != PhaseDusk || g.Dusk.Resolved {
			return errors.New("not now")
		}
		g.resolveDusk()
	case "nextSeason":
		if g.Phase != PhaseDusk || !g.Dusk.Resolved {
			return errors.New("set the Dusk pairs first")
		}
		g.nextSeason()
	case "startChain":
		if g.Phase != PhaseGuess {
			return errors.New("not now")
		}
		g.Phase = PhaseChain
		g.ChainIdx = 0
		g.setTimer(60, "Tribute")
	case "nextTribute":
		if g.Phase != PhaseChain {
			return errors.New("not now")
		}
		g.advanceChain()
	case "timer":
		g.setTimer(a.N, "Timer")
	default:
		return errors.New("unknown host action")
	}
	return nil
}

func (g *Game) advanceChain() {
	g.ChainIdx++
	if g.ChainIdx >= len(g.Chain) {
		g.Phase = PhaseScores
		g.TimerEnd, g.TimerLabel = 0, ""
		g.logf("The forest is grown. Final scores!")
		return
	}
	g.setTimer(60, "Tribute")
}

func (g *Game) playerAction(me *Player, a Action) error {
	switch a.Type {
	case "plant":
		if g.Phase != PhasePlant || g.current().ID != me.ID {
			return errors.New("not your turn to plant")
		}
		if a.Hex < 0 || a.Hex >= len(g.Hexes) || g.Hexes[a.Hex].Ring != 3 {
			return errors.New("plant on an outer Seedlands space")
		}
		me.Pos = a.Hex
		g.logf("%s planted their seed in %s", me.Name, RegionNames[g.Hexes[a.Hex].Region])
		g.TurnsTaken++
		if g.TurnsTaken >= len(g.Order) {
			g.Phase = PhaseTurn
			g.Season = 1
			g.TurnIdx, g.TurnsTaken, g.StartIdx = 0, 0, 0
			g.newTurn()
			g.logf("Season 1: %s", SeasonNames[1])
		} else {
			g.TurnIdx = (g.TurnIdx + 1) % len(g.Order)
		}
		return nil
	case "trust":
		return g.giveTrust(me, 1)
	case "squirrel":
		if g.Phase != PhaseTurn || g.Turn.Step != "trust" || g.Turn.Player == me.ID {
			return errors.New("play a Squirrel right after someone else's share")
		}
		if me.Squirrels < 1 {
			return errors.New("you have no Squirrel")
		}
		me.Squirrels--
		me.Bonus++
		q := SquirrelCards[rand.Intn(len(SquirrelCards))]
		g.Turn.FollowUps = append(g.Turn.FollowUps, me.Name+" asks: "+q)
		g.setTimer(30, "Follow-up")
		return nil
	case "duskChoice":
		if g.Phase != PhaseDusk || g.Dusk.Resolved {
			return errors.New("not Dusk")
		}
		if a.Target == me.ID {
			return errors.New("choose someone else")
		}
		if a.Target != "" {
			if g.player(a.Target) == nil {
				return errors.New("no such player")
			}
			if contains(me.Partners, a.Target) {
				return errors.New("you've already paired with them; choose someone new")
			}
		}
		g.Dusk.Choices[me.ID] = DuskChoice{Partner: a.Target, Ally: a.Ally}
		return nil
	case "guess":
		if g.Phase != PhaseGuess {
			return errors.New("not now")
		}
		if a.Target == me.ID || g.player(a.Target) == nil {
			return errors.New("guess someone else")
		}
		me.Guess = a.Target
		return nil
	case "confirm":
		if g.Phase != PhaseChain {
			return errors.New("not now")
		}
		giver := g.player(g.Chain[g.ChainIdx])
		if giver.Target != me.ID {
			return errors.New("only the person receiving the tribute confirms")
		}
		if a.N < 0 || a.N > 3 {
			return errors.New("0 to 3")
		}
		giver.Confirmed = a.N
		g.logf("%s honoured %s", giver.Name, me.Name)
		g.advanceChain()
		return nil
	case "power":
		return g.usePower(me, a)
	}

	// Everything below is the active player's own turn.
	if g.Phase != PhaseTurn || g.Turn == nil || g.Turn.Player != me.ID {
		return errors.New("it's not your turn")
	}
	tu := g.Turn
	switch a.Type {
	case "step":
		if tu.Step != "move" {
			return errors.New("you've finished moving")
		}
		if a.Hex < 0 || a.Hex >= len(g.Hexes) {
			return errors.New("no such space")
		}
		if tu.Teleport {
			h := g.Hexes[a.Hex]
			if h.Ring != g.Hexes[me.Pos].Ring || !g.ringOpen(h.Ring) {
				return errors.New("the hidden path leads to a space in this ring")
			}
			me.Pos = a.Hex
			tu.Teleport = false
			g.toShare(me)
			return nil
		}
		if tu.Discovered {
			return errors.New("you've finished moving")
		}
		if g.dist(me.Pos, a.Hex) != 1 {
			return errors.New("step to a neighbouring space")
		}
		if tu.StepsLeft < 1 {
			return errors.New("no steps left this turn")
		}
		if err := g.canEnter(me, a.Hex); err != nil {
			return err
		}
		tu.StepsLeft--
		me.Pos = a.Hex
		tu.CanClaim = false // scouting is about the spaces next to where you used it
		me.Peek = map[int]string{}
	case "endMove":
		if tu.Step != "move" {
			return errors.New("already done")
		}
		if !tu.Discovered {
			tu.Discovered = true
			g.resolveToken(me, me.Pos)
			if tu.Teleport {
				return nil
			}
		}
		g.toShare(me)
	case "claim":
		if !tu.CanClaim || me.Peek[a.Hex] == "" {
			return errors.New("claim one of the discoveries you scouted")
		}
		tu.CanClaim = false
		me.Peek = map[int]string{}
		disc := tu.Discovered
		g.resolveToken(me, a.Hex)
		tu.Discovered = disc
	case "redraw":
		if tu.Step != "share" || tu.Started || tu.Redrawn || len(tu.Choices) > 0 {
			return errors.New("you can draw again once, before you start")
		}
		tu.Redrawn = true
		if tu.Tier == 4 {
			tu.Card = g.draw(6)
		} else {
			tu.Card = g.draw(tu.Region)
		}
		g.setPrompt()
	case "lighter":
		if tu.Step != "share" || tu.Started || tu.Tier <= 1 || tu.Tier >= 4 {
			return errors.New("no lighter tier available")
		}
		tu.Tier--
		g.setPrompt()
	case "pickChoice":
		found := false
		for _, c := range tu.Choices {
			if c == a.Card {
				found = true
			}
		}
		if !found {
			return errors.New("pick one of the three cards")
		}
		tu.Card = a.Card
		tu.Choices = nil
		g.setPrompt()
	case "pass":
		if tu.Started || tu.Step == "trust" {
			return errors.New("you've already shared this turn")
		}
		g.logf("%s passed this turn", me.Name)
		tu.Prompt = ""
		g.finishTurn()
	case "startShare":
		if tu.Step != "share" || tu.Started || len(tu.Choices) > 0 {
			return errors.New("not now")
		}
		tu.Started = true
		g.setTimer(shareSecs(tu.Tier), "Sharing")
	case "doneShare":
		if tu.Step != "share" || !tu.Started {
			return errors.New("start sharing first")
		}
		tu.Step = "trust"
		g.TimerEnd, g.TimerLabel = 0, ""
	case "endTrust":
		if tu.Step != "trust" {
			return errors.New("not collecting trust")
		}
		g.finishTurn()
	default:
		return errors.New("unknown action")
	}
	return nil
}

func (g *Game) giveTrust(me *Player, n int) error {
	if g.Phase != PhaseTurn || g.Turn == nil || g.Turn.Step != "trust" {
		return errors.New("trust is given right after a share")
	}
	if g.Turn.Player == me.ID {
		return errors.New("you can't trust yourself")
	}
	if g.Turn.Trusted[me.ID] > 0 {
		return errors.New("you've already given trust for this share")
	}
	if me.TrustLeft < n {
		return errors.New("not enough trust acorns left")
	}
	me.TrustLeft -= n
	g.current().Pot[me.ID] += n
	g.Turn.Trusted[me.ID] = n
	return nil
}

func (g *Game) usePower(me *Player, a Action) error {
	t := a.Power
	owned := false
	for _, x := range me.Types {
		if x == t {
			owned = true
		}
	}
	if !owned {
		return errors.New("that isn't one of your powers")
	}
	if me.Used[t] {
		return errors.New("you've already used that power")
	}
	tu := g.Turn
	myTurn := g.Phase == PhaseTurn && tu != nil && tu.Player == me.ID
	moving := myTurn && tu.Step == "move" && !tu.Discovered
	beforeShare := myTurn && tu.Step == "share" && !tu.Started
	switch t {
	case 1:
		if !beforeShare || tu.Tier == 4 || len(tu.Choices) > 0 {
			return errors.New("use True North on your turn, before you start sharing (not in the Heartwood)")
		}
		tu.Choices = []int{tu.Card, g.draw(tu.Region), g.draw(tu.Region)}
		tu.Bonus++
	case 2:
		if !beforeShare || len(tu.Choices) > 0 {
			return errors.New("use Open Hands on your turn, before you start sharing")
		}
		o := g.player(a.Target)
		if o == nil || o.ID == me.ID {
			return errors.New("choose another player")
		}
		tu.Invited = o.ID
		o.Bonus++
		me.Bonus++
		g.logf("%s invited %s to answer the question too", me.Name, o.Name)
	case 3:
		if !moving {
			return errors.New("use Momentum while moving")
		}
		tu.StepsLeft += 3
	case 4:
		if !beforeShare || tu.Tier >= 3 {
			return errors.New("use Deep Water before sharing, below the Oak tier")
		}
		// The deeper question is the reward; it scores at the ring's tier.
		if tu.ScoreTier == 0 {
			tu.ScoreTier = tu.Tier
		}
		tu.Tier++
		g.setPrompt()
	case 5:
		if !moving {
			return errors.New("use Field Notes while moving")
		}
		me.Peek = map[int]string{}
		for i := range g.Hexes {
			if g.dist(me.Pos, i) == 1 && g.ringOpen(g.Hexes[i].Ring) {
				if tok := g.Tokens[i]; tok != nil && !tok.Flipped {
					me.Peek[i] = tok.Kind
				}
			}
		}
		if len(me.Peek) == 0 {
			return errors.New("no hidden discoveries next to you")
		}
		tu.CanClaim = true
	case 6:
		if !moving {
			return errors.New("use Rope Team while moving")
		}
		o := g.player(a.Target)
		if o == nil || !hasAlly(me, o.ID) {
			return errors.New("choose one of your allies")
		}
		if !g.ringOpen(g.Hexes[o.Pos].Ring) {
			return errors.New("that ring isn't open yet")
		}
		me.Pos = o.Pos
		me.Bonus++
		o.Bonus++
	case 7:
		if !myTurn || tu.Step == "trust" {
			return errors.New("use Adventure on your turn, before your share is done")
		}
		tu.Campfire = CampfireCards[rand.Intn(len(CampfireCards))]
		me.Bonus++
		g.logf("%s called a Campfire: %s", me.Name, tu.Campfire)
	case 8:
		if err := g.giveTrust(me, 2); err != nil {
			return err
		}
		me.Bonus++
	case 9:
		if g.Phase != PhaseDusk || g.Dusk.Resolved {
			return errors.New("use Common Ground at Dusk, before pairs are set")
		}
		g.Dusk.Common[me.ID] = true
	default:
		return errors.New("unknown power")
	}
	me.Used[t] = true
	g.logf("%s used %s (%s)", me.Name, Powers[t-1].Title, Powers[t-1].Name)
	return nil
}
