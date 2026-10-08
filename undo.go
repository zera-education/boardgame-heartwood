package main

import (
	"encoding/json"
	"errors"
)

// Undo: the Keeper taps every public action for the players, and a tap can
// slip (the wrong hex, the wrong player's move). Before each such tap the game
// keeps a copy of itself; "undo" puts the newest copy back. The copies last
// until the turn ends, so only this turn's taps can be taken back.

// MaxUndo is how many taps back the Keeper can go.
const MaxUndo = 6

// UndoPoint is the game just before one Keeper tap.
type UndoPoint struct {
	Label string          `json:"label"` // what the tap did, as the trail log said it
	State json.RawMessage `json:"state"` // the game before it, without its own undo points
}

// keeperTap applies a play action the Keeper taps for a player, keeping an
// undo point when it works. Ending the turn starts the next one fresh: it is
// not taken back, and neither is a tap that moves the game on (the last
// player entering starts play; a tap that wakes the forest ends it).
func (g *Game) keeperTap(p *Player, a Action) error {
	if a.Type == "endTurn" {
		return g.playerAction(p, a)
	}
	undo := g.Undo
	g.Undo = nil
	state, err := json.Marshal(g)
	g.Undo = undo
	if err != nil {
		return err
	}
	logged, phase := g.logged, g.Phase
	if err := g.playerAction(p, a); err != nil {
		return err
	}
	if g.Phase != phase {
		return nil
	}
	label := p.Name + ": " + a.Type
	if n := g.logged - logged; n > 0 && n <= len(g.Log) {
		label = g.Log[len(g.Log)-n]
	}
	g.Undo = append(g.Undo, UndoPoint{Label: label, State: state})
	if len(g.Undo) > MaxUndo {
		g.Undo = g.Undo[len(g.Undo)-MaxUndo:]
	}
	return nil
}

// undo puts back the game as it was before the Keeper's last tap this turn.
// What isn't play stays as it is now: the version, the event and share
// numbers (they only ever grow, so screens and recordings stay in step),
// photos, rejoin codes and the recording settings. Players hop back on the
// big screen.
func (g *Game) undo() error {
	if len(g.Undo) == 0 || (g.Phase != PhaseEnter && g.Phase != PhaseTurn) {
		return errors.New("nothing to undo")
	}
	u := g.Undo[len(g.Undo)-1]
	var old Game
	if err := json.Unmarshal(u.State, &old); err != nil {
		return err
	}
	now := *g
	*g = old
	g.Undo = now.Undo[:len(now.Undo)-1]
	g.Version, g.EventSeq, g.ShareSeq, g.logged = now.Version, now.EventSeq, now.ShareSeq, now.logged
	g.Rejoin, g.NoRecord, g.RecDevice = now.Rejoin, now.NoRecord, now.RecDevice
	// the same Player values as before, so nothing that holds one goes stale
	for i, p := range g.Players {
		q := now.player(p.ID)
		if q == nil {
			continue
		}
		from, photo := q.Pos, q.Photo
		*q = *p
		q.Photo = photo
		g.Players[i] = q
		if from >= 0 && q.Pos >= 0 && from != q.Pos {
			g.event("move", Event{"pid": q.ID, "from": from, "to": q.Pos, "undo": true})
		}
	}
	g.logf("The Keeper took back: %s", u.Label)
	return nil
}
