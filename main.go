package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"math/big"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	_ "modernc.org/sqlite"
	"rsc.io/qr"
)

//go:embed web
var webFS embed.FS

func randID(n int) string {
	const chars = "abcdefghjkmnpqrstuvwxyz23456789"
	b := make([]byte, n)
	for i := range b {
		x, _ := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		b[i] = chars[x.Int64()]
	}
	return string(b)
}

func randPin() string {
	x, _ := rand.Int(rand.Reader, big.NewInt(9000))
	return fmt.Sprint(1000 + x.Int64())
}

type Server struct {
	mu    sync.Mutex
	db    *sql.DB
	games map[string]*Game
	subs  map[string]map[chan struct{}]bool
}

// lanURLs lists this computer's Wi-Fi/LAN addresses, so the board can show
// phones a join link that works even when the Keeper opened it at localhost.
func lanURLs(addr string) []string {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil
	}
	var out []string
	ifaces, _ := net.Interfaces()
	for _, ifc := range ifaces {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := ifc.Addrs()
		for _, a := range addrs {
			ipn, ok := a.(*net.IPNet)
			if !ok || ipn.IP.To4() == nil || ipn.IP.IsLinkLocalUnicast() {
				continue
			}
			out = append(out, fmt.Sprintf("http://%s:%s", ipn.IP, port))
		}
	}
	return out
}

var lan []string

// qrCode draws a QR code for a join link, as a PNG.
func qrCode(w http.ResponseWriter, r *http.Request) {
	u := r.URL.Query().Get("u")
	if len(u) > 300 || !(strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://")) {
		http.Error(w, "bad link", 400)
		return
	}
	c, err := qr.Encode(u, qr.M)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	c.Scale = 8
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "max-age=86400")
	w.Write(c.PNG())
}

func openDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`
CREATE TABLE IF NOT EXISTS games (code TEXT PRIMARY KEY, state TEXT NOT NULL, updated_at INTEGER NOT NULL);
CREATE TABLE IF NOT EXISTS events (id INTEGER PRIMARY KEY AUTOINCREMENT, code TEXT NOT NULL, actor TEXT, type TEXT, payload TEXT, at INTEGER NOT NULL);`)
	return db, err
}

func (s *Server) load() error {
	rows, err := s.db.Query(`SELECT code, state FROM games`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var code, state string
		if err := rows.Scan(&code, &state); err != nil {
			return err
		}
		g := &Game{}
		if err := json.Unmarshal([]byte(state), g); err != nil {
			log.Printf("skip game %s: %v", code, err)
			continue
		}
		s.games[code] = g
	}
	return rows.Err()
}

func (s *Server) save(g *Game) error {
	b, err := json.Marshal(g)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`INSERT INTO games(code,state,updated_at) VALUES(?,?,?)
ON CONFLICT(code) DO UPDATE SET state=excluded.state, updated_at=excluded.updated_at`, g.Code, string(b), time.Now().Unix())
	return err
}

// changed bumps the version, saves, and wakes every live connection to the game.
func (s *Server) changed(g *Game) error {
	g.Version++
	if err := s.save(g); err != nil {
		return err
	}
	for ch := range s.subs[g.Code] {
		select {
		case ch <- struct{}{}:
		default: // a send is already pending; it will carry the latest state
		}
	}
	return nil
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func fail(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func (s *Server) game(w http.ResponseWriter, r *http.Request) *Game {
	g := s.games[strings.ToUpper(r.PathValue("code"))]
	if g == nil {
		fail(w, 404, "no game with that code")
	}
	return g
}

func (s *Server) create(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	code := ""
	for code == "" || s.games[code] != nil {
		code = strings.ToUpper(randID(4))
	}
	g := NewGame(code, randID(16))
	s.games[code] = g
	if err := s.changed(g); err != nil {
		fail(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]string{"code": code, "host": g.HostSecret})
}

func (s *Server) join(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name  string `json:"name"`
		Color string `json:"color"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, 400, "bad request")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	g := s.game(w, r)
	if g == nil {
		return
	}
	p, err := g.Join(strings.TrimSpace(req.Name), req.Color)
	if err != nil {
		fail(w, 400, err.Error())
		return
	}
	s.changed(g)
	writeJSON(w, 200, map[string]string{"pid": p.ID, "secret": p.Secret, "name": p.Name})
}

// rejoinCode lets the Keeper hand a player a 4-digit code to recover their seat
// on another phone or browser. Codes last 10 minutes and work once.
func (s *Server) rejoinCode(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Host string `json:"host"`
		Pid  string `json:"pid"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, 400, "bad request")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	g := s.game(w, r)
	if g == nil {
		return
	}
	if req.Host == "" || req.Host != g.HostSecret {
		fail(w, 403, "only the Keeper can do that")
		return
	}
	p := g.player(req.Pid)
	if p == nil {
		fail(w, 404, "no such player")
		return
	}
	now := time.Now().Unix()
	for pin, rp := range g.Rejoin {
		if rp.Expires < now || rp.Pid == p.ID {
			delete(g.Rejoin, pin)
		}
	}
	if g.Rejoin == nil {
		g.Rejoin = map[string]RejoinPin{}
	}
	pin := randPin()
	for g.Rejoin[pin].Pid != "" {
		pin = randPin()
	}
	g.Rejoin[pin] = RejoinPin{Pid: p.ID, Expires: now + 600}
	s.save(g)
	writeJSON(w, 200, map[string]string{"pin": pin, "name": p.Name})
}

func (s *Server) rejoin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Pin string `json:"pin"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, 400, "bad request")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	g := s.game(w, r)
	if g == nil {
		return
	}
	rp, ok := g.Rejoin[strings.TrimSpace(req.Pin)]
	if !ok || rp.Expires < time.Now().Unix() {
		fail(w, 400, "that code didn't work; ask the Keeper for a new one")
		return
	}
	delete(g.Rejoin, req.Pin)
	p := g.player(rp.Pid)
	if p == nil {
		fail(w, 404, "no such player")
		return
	}
	g.logf("%s rejoined", p.Name)
	s.changed(g)
	writeJSON(w, 200, map[string]string{"pid": p.ID, "secret": p.Secret, "name": p.Name})
}

func (s *Server) action(w http.ResponseWriter, r *http.Request) {
	var a Action
	a.Hex = -1
	if err := json.NewDecoder(r.Body).Decode(&a); err != nil {
		fail(w, 400, "bad request")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	g := s.game(w, r)
	if g == nil {
		return
	}
	if err := g.Apply(a); err != nil {
		fail(w, 400, err.Error())
		return
	}
	payload, _ := json.Marshal(map[string]any{"hex": a.Hex, "target": a.Target, "power": a.Power, "n": a.N, "types": a.Types, "as": a.As})
	actor := a.Pid
	if a.Host != "" {
		actor = "host"
	}
	s.db.Exec(`INSERT INTO events(code,actor,type,payload,at) VALUES(?,?,?,?,?)`, g.Code, actor, a.Type, string(payload), time.Now().Unix())
	if err := s.changed(g); err != nil {
		fail(w, 500, err.Error())
		return
	}
	// The actor gets their new view straight back, without waiting for the push.
	writeJSON(w, 200, map[string]any{"ok": true, "view": buildView(g, a.Pid, a.Secret, a.Host)})
}

func (s *Server) state(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	g := s.game(w, r)
	if g == nil {
		return
	}
	q := r.URL.Query()
	writeJSON(w, 200, buildView(g, q.Get("pid"), q.Get("secret"), q.Get("host")))
}

func (s *Server) subscribe(code string) chan struct{} {
	ch := make(chan struct{}, 1)
	if s.subs[code] == nil {
		s.subs[code] = map[chan struct{}]bool{}
	}
	s.subs[code][ch] = true
	return ch
}

// live is the realtime connection. A WebSocket (not SSE): browsers allow only
// six long-lived HTTP/1.1 connections per host, so a board plus five player
// tabs on one computer used to freeze every other request.
//
// The server pushes this viewer's whole view after every change, in order, and
// a small heartbeat every 20 s so clients can spot a dead connection. Any
// message from the client (it sends "sync" when a phone wakes up) asks for a
// fresh view.
func (s *Server) live(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	g := s.game(w, r)
	if g == nil {
		s.mu.Unlock()
		return
	}
	code := g.Code
	ch := s.subscribe(code)
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.subs[code], ch)
		s.mu.Unlock()
	}()

	c, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer c.CloseNow()
	q := r.URL.Query()
	pid, secret, host := q.Get("pid"), q.Get("secret"), q.Get("host")

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	go func() {
		defer cancel()
		for {
			if _, _, err := c.Read(ctx); err != nil {
				return
			}
			select {
			case ch <- struct{}{}:
			default:
			}
		}
	}()
	write := func(b []byte) error {
		wctx, wcancel := context.WithTimeout(ctx, 10*time.Second)
		defer wcancel()
		return c.Write(wctx, websocket.MessageText, b)
	}
	send := func() error {
		s.mu.Lock()
		var b []byte
		if g := s.games[code]; g == nil {
			b = []byte(`{"error":"no game with that code"}`)
		} else {
			b, _ = json.Marshal(buildView(g, pid, secret, host))
		}
		s.mu.Unlock()
		return write(b)
	}
	if send() != nil {
		return
	}
	beat := time.NewTicker(20 * time.Second)
	defer beat.Stop()
	for {
		select {
		case <-ctx.Done():
			c.Close(websocket.StatusNormalClosure, "")
			return
		case <-ch:
			if send() != nil {
				return
			}
		case <-beat.C:
			if write([]byte(`{"ping":1}`)) != nil {
				return
			}
		}
	}
}

// buildView returns what one viewer may see: secrets, hidden tokens and
// other players' pots stay on the server.
func buildView(g *Game, pid, secret, host string) map[string]any {
	isHost := host != "" && host == g.HostSecret
	var me *Player
	if p := g.player(pid); p != nil && p.Secret == secret {
		me = p
	}
	players := []map[string]any{}
	for _, p := range g.Players {
		pp := map[string]any{
			"id": p.ID, "name": p.Name, "color": p.Color, "types": p.Types, "used": p.Used,
			"pos": p.Pos, "bonus": p.Bonus, "trustLeft": p.TrustLeft,
			"squirrels": p.Squirrels, "cards": p.Cards, "allies": p.Allies, "partners": p.Partners,
			"guessed": p.Guess != "",
		}
		if g.Dusk != nil {
			_, pp["duskChosen"] = g.Dusk.Choices[p.ID]
		}
		players = append(players, pp)
	}
	tokens := map[int]string{}
	for i, t := range g.Tokens {
		if t.Flipped {
			tokens[i] = t.Kind
		} else {
			tokens[i] = ""
		}
	}
	v := map[string]any{
		"code": g.Code, "phase": g.Phase, "season": g.Season, "seasonName": SeasonNames[min(g.Season, 3)],
		"forestGoal": g.ForestGoal, "players": players, "order": g.Order, "hexes": g.Hexes,
		"tokens": tokens, "trees": g.Trees, "oak": g.Oak, "timerEnd": g.TimerEnd, "timerLabel": g.TimerLabel,
		"now": time.Now().UnixMilli(), "version": g.Version, "isHost": isHost,
		"powers": Powers, "regions": RegionNames, "colors": Colors, "lan": lan,
	}
	if n := len(g.Log); n > 40 {
		v["log"] = g.Log[n-40:]
	} else {
		v["log"] = g.Log
	}
	if (g.Phase == PhasePlant || g.Phase == PhaseTurn) && len(g.Order) > 0 {
		v["current"] = g.Order[g.TurnIdx]
	}
	if t := g.Turn; t != nil && g.Phase == PhaseTurn {
		n := 0
		for _, c := range t.Trusted {
			n += c
		}
		tv := map[string]any{
			"player": t.Player, "step": t.Step, "discovered": t.Discovered, "stepsLeft": t.StepsLeft,
			"teleport": t.Teleport, "canClaim": t.CanClaim, "region": t.Region, "tier": t.Tier,
			"prompt": t.Prompt, "redrawn": t.Redrawn, "started": t.Started, "event": t.Event,
			"eventText":  t.EventText,
			"trustCount": n, "followUps": t.FollowUps,
			"invited": t.Invited, "campfire": t.Campfire,
		}
		if me != nil {
			tv["iGave"] = t.Trusted[me.ID] > 0
		}
		if isHost {
			// The Keeper runs Field Notes on the big screen, so it sees what was scouted.
			if p := g.player(t.Player); p != nil {
				tv["peek"] = p.Peek
			}
		}
		var choices []string
		for _, c := range t.Choices {
			choices = append(choices, RegionCards[t.Region][c][t.Tier-1])
		}
		tv["choices"] = choices
		tv["choiceIds"] = t.Choices
		v["turn"] = tv
	}
	if d := g.Dusk; d != nil {
		v["dusk"] = map[string]any{"pairs": d.Pairs, "resolved": d.Resolved}
	}
	if g.Phase == PhaseChain && g.ChainIdx < len(g.Chain) {
		giver := g.player(g.Chain[g.ChainIdx])
		v["tribute"] = map[string]any{"giver": giver.ID, "receiver": giver.Target, "idx": g.ChainIdx, "total": len(g.Chain)}
	}
	if g.Phase == PhaseScores {
		scores, stands := g.Scores()
		v["scores"] = scores
		v["forestStands"] = stands
	}
	if me != nil {
		pot, givers := 0, 0
		for _, c := range me.Pot {
			pot += c
			if c > 0 {
				givers++
			}
		}
		mv := map[string]any{
			"id": me.ID, "target": me.Target, "guess": me.Guess, "peek": me.Peek,
			"potCount": pot, "potGivers": givers,
		}
		if t := g.player(me.Target); t != nil {
			mv["targetName"] = t.Name
		}
		if g.Dusk != nil {
			if c, ok := g.Dusk.Choices[me.ID]; ok {
				mv["duskChoice"] = c
			}
			mv["common"] = g.Dusk.Common[me.ID]
		}
		v["me"] = mv
	}
	return v
}

func main() {
	addr := flag.String("addr", ":8490", "listen address")
	dbPath := flag.String("db", "heartwood.db", "SQLite file")
	flag.Parse()
	db, err := openDB(*dbPath)
	if err != nil {
		log.Fatal(err)
	}
	s := &Server{db: db, games: map[string]*Game{}, subs: map[string]map[chan struct{}]bool{}}
	if err := s.load(); err != nil {
		log.Fatal(err)
	}
	lan = lanURLs(*addr)
	sub, _ := fs.Sub(webFS, "web")
	static := http.FileServerFS(sub)
	mux := http.NewServeMux()
	mux.Handle("GET /", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache") // always pick up a rebuilt UI
		static.ServeHTTP(w, r)
	}))
	mux.HandleFunc("POST /api/games", s.create)
	mux.HandleFunc("POST /api/games/{code}/join", s.join)
	mux.HandleFunc("POST /api/games/{code}/rejoin", s.rejoin)
	mux.HandleFunc("POST /api/games/{code}/rejoin-code", s.rejoinCode)
	mux.HandleFunc("POST /api/games/{code}/action", s.action)
	mux.HandleFunc("GET /api/games/{code}/state", s.state)
	mux.HandleFunc("GET /api/games/{code}/live", s.live)
	mux.HandleFunc("GET /api/qr", qrCode)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { fmt.Fprintln(w, "ok") })
	log.Printf("Heartwood on http://localhost%s (db %s, %d saved games)", *addr, *dbPath, len(s.games))
	for _, u := range lan {
		log.Printf("Phones on the same Wi-Fi: %s", u)
	}
	log.Fatal(http.ListenAndServe(*addr, mux))
}
