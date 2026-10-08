package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"unicode"
)

// A sealed find is a one-off surprise for one player: a picture the forest
// turns up when they explore, which every screen then shows while the forest
// asks them about it. What it is and whom it waits for are private, so they
// never enter this (public) repository: the server keeps them in a file beside
// its database, put there or taken away by the Mini over the worker's token
// (PUT or DELETE /api/found). Without that file, explore is as it always was.

// SealedFind is that file.
type SealedFind struct {
	Name     string `json:"name"`     // the first name it waits for
	Question string `json:"question"` // what the forest asks them; a default when empty
	Cheer    string `json:"cheer"`    // the line once they've explained
	Image    []byte `json:"image"`    // JPEG or PNG (base64 in the file)
}

// FoundQuestion is what the forest asks unless the find says otherwise.
const FoundQuestion = "Why is this picture here? Can you explain?"

var (
	sealed    atomic.Pointer[SealedFind]
	sealedVer atomic.Int64 // bumps on every change, so screens fetch the new picture
)

// calls tells whether a player of this name is the one the find waits for:
// their first name, whatever the case.
func (f *SealedFind) calls(name string) bool {
	first, _, _ := strings.Cut(strings.TrimSpace(name), " ")
	first = strings.TrimFunc(first, func(r rune) bool { return !unicode.IsLetter(r) })
	return f != nil && f.Name != "" && strings.EqualFold(first, f.Name)
}

func (f *SealedFind) question() string {
	if f.Question != "" {
		return f.Question
	}
	return FoundQuestion
}

// Found is the sealed find turned up in this game: once per game. Stage ask is
// the forest's question, cheer the celebration after they've explained, done
// when the Keeper closes it.
type Found struct {
	Player string `json:"player"`
	Hex    int    `json:"hex"`
	Stage  string `json:"stage"`
}

// FoundAfter is the explore of that player that turns the find up: not their
// first, their third (El, 2026-10-08), or the next empty hex after it.
const FoundAfter = 3

// turnUp is a player exploring a hex of this kind: the sealed find, if this is
// its player, it hasn't been found in this game yet, this is their third
// explore or later, and the hex is empty.
func (g *Game) turnUp(me *Player, kind string) bool {
	if g.Found != nil || !sealed.Load().calls(me.Name) {
		return false
	}
	g.FoundTries++
	if g.FoundTries < FoundAfter || kind != "empty" {
		return false
	}
	g.Found = &Found{Player: me.ID, Hex: me.Pos, Stage: "ask"}
	g.logf("%s explores and finds something… unexpected!?", me.Name)
	return true
}

// foundStage is the Keeper moving the find on: to the celebration, or closed.
func (g *Game) foundStage(stage string) error {
	if g.Found == nil || g.Found.Stage == "done" {
		return errors.New("nothing has been found")
	}
	g.Found.Stage = stage
	return nil
}

// foundURL is the find's picture for this game's screens.
func foundURL(g *Game) string {
	return fmt.Sprintf("/api/games/%s/found?v=%d", g.Code, sealedVer.Load())
}

// foundBy is the picture's URL if this player turned the find up (the
// recognition shows it), or "".
func (g *Game) foundBy(pid string) string {
	if g.Found == nil || g.Found.Player != pid || sealed.Load() == nil {
		return ""
	}
	return foundURL(g)
}

// foundView is what every screen gets while the find is showing.
func foundView(g *Game) map[string]any {
	f := sealed.Load()
	if f == nil || g.Found == nil || g.Found.Stage == "done" {
		return nil
	}
	return map[string]any{"player": g.Found.Player, "stage": g.Found.Stage, "question": f.question(), "cheer": f.Cheer,
		"image": foundURL(g)}
}

// ---- the file and its API ----

// findPath is where the sealed find lives: beside the database.
func findPath(dbPath string) string { return filepath.Join(filepath.Dir(dbPath), "sealed-find.json") }

func loadFind(path string) error {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		sealed.Store(nil)
		return nil
	}
	if err != nil {
		return err
	}
	var f SealedFind
	if err := json.Unmarshal(data, &f); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if err := f.check(); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	sealed.Store(&f)
	sealedVer.Add(1)
	return nil
}

func (f *SealedFind) check() error {
	if strings.TrimSpace(f.Name) == "" {
		return errors.New("a sealed find needs the name it waits for")
	}
	if ct := http.DetectContentType(f.Image); ct != "image/jpeg" && ct != "image/png" {
		return errors.New("a sealed find needs a JPEG or PNG picture")
	}
	return nil
}

// putFind stores a new sealed find (the worker's token).
//
// PUT /api/found  {"name": "…", "question": "…", "cheer": "…", "image": "<base64>"}
func (s *Server) putFind(w http.ResponseWriter, r *http.Request) {
	if !s.worker(w, r) {
		return
	}
	var f SealedFind
	if err := json.NewDecoder(io.LimitReader(r.Body, 8<<20)).Decode(&f); err != nil {
		fail(w, 400, "bad request")
		return
	}
	if err := f.check(); err != nil {
		fail(w, 400, err.Error())
		return
	}
	data, _ := json.Marshal(f)
	tmp := s.findPath + ".new"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		fail(w, 500, err.Error())
		return
	}
	if err := os.Rename(tmp, s.findPath); err != nil {
		fail(w, 500, err.Error())
		return
	}
	sealed.Store(&f)
	sealedVer.Add(1)
	writeJSON(w, 200, map[string]any{"ok": true, "bytes": len(f.Image)})
}

// deleteFind removes the sealed find; games that turned it up stop showing it.
//
// DELETE /api/found
func (s *Server) deleteFind(w http.ResponseWriter, r *http.Request) {
	if !s.worker(w, r) {
		return
	}
	if err := os.Remove(s.findPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		fail(w, 500, err.Error())
		return
	}
	sealed.Store(nil)
	sealedVer.Add(1)
	writeJSON(w, 200, map[string]any{"ok": true})
}

// foundImage is the picture, for the screens of a game that turned it up.
//
// GET /api/games/{code}/found?v=N
func (s *Server) foundImage(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	g := s.game(w, r)
	found := g != nil && g.Found != nil
	s.mu.Unlock()
	if g == nil {
		return
	}
	f := sealed.Load()
	if !found || f == nil {
		fail(w, 404, "nothing here")
		return
	}
	w.Header().Set("Content-Type", http.DetectContentType(f.Image))
	w.Header().Set("Cache-Control", "private, max-age=3600")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Write(f.Image)
}

func (s *Server) findRoutes(mux *http.ServeMux) {
	mux.HandleFunc("PUT /api/found", s.putFind)
	mux.HandleFunc("DELETE /api/found", s.deleteFind)
	mux.HandleFunc("GET /api/games/{code}/found", s.foundImage)
}
