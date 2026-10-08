package main

// The admin: one private page (web/admin.html, also at /admin) where El sees
// every game on the server and manages it: runs it as Keeper on the device in
// his hand (the Keeper link), adds bot seats and removes players in the lobby,
// ends a game early and deletes old ones.
//
// Only El gets in. He asks for a code; the server sends a 6-digit code to his
// Telegram through the Bot API and keeps only its hash. It lasts 5 minutes,
// works once, a new one can be sent every 30 s, and 5 wrong tries void it. A
// right code gives this browser a session cookie for 30 days (HttpOnly,
// Secure, SameSite=Strict), kept hashed in SQLite. Every /api/admin route but
// the login ones checks it.
//
// The bot token and El's chat id come from environment variables whose names
// are flags (-tg-token-var, -tg-chat-var), so the server never holds them in
// its code or its database. With either one missing, the login says Telegram
// isn't set up and everything else works as before.
//
// Deleting a game keeps its story recordings (the team's record): the game
// then stays in the list as "stories only" (table deleted_games, which also
// keeps its Keeper secret, so its Stories page still opens).

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"log"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	adminCodeLife    = 5 * time.Minute
	adminCodeGap     = 30 * time.Second // one code per 30 s
	adminCodesHour   = 6                // and at most 6 an hour: 30 guesses an hour in a million
	adminCodeTries   = 5                // wrong tries before every waiting code is void
	adminSessionLife = 30 * 24 * time.Hour
	adminCookie      = "hw_admin"
)

// telegram is where login codes go: the Bot API, with the token and chat id
// read from the environment at send time.
type telegramConfig struct {
	TokenVar string // name of the env var holding the bot token
	ChatVar  string // name of the env var holding El's chat id
	API      string // Bot API base URL (a fake one in tests)
}

var tg = telegramConfig{TokenVar: "HEARTWOOD_TG_TOKEN", ChatVar: "HEARTWOOD_TG_CHAT", API: "https://api.telegram.org"}

func (c telegramConfig) creds() (token, chat string) {
	return strings.TrimSpace(os.Getenv(c.TokenVar)), strings.TrimSpace(os.Getenv(c.ChatVar))
}

func (c telegramConfig) ready() bool {
	token, chat := c.creds()
	return token != "" && chat != ""
}

var errTelegramOff = errors.New("Telegram isn't set up on this server yet")

var tgClient = &http.Client{Timeout: 10 * time.Second}

// send posts one message to El's chat. Errors never carry the token (a
// url.Error would print the whole request URL).
func (c telegramConfig) send(text string) error {
	token, chat := c.creds()
	if token == "" || chat == "" {
		return errTelegramOff
	}
	body, _ := json.Marshal(map[string]any{"chat_id": chat, "text": text, "parse_mode": "HTML", "disable_web_page_preview": true})
	res, err := tgClient.Post(strings.TrimRight(c.API, "/")+"/bot"+token+"/sendMessage", "application/json", bytes.NewReader(body))
	if err != nil {
		var ue *url.Error
		if errors.As(err, &ue) {
			err = ue.Err
		}
		return fmt.Errorf("couldn't reach Telegram: %v", err)
	}
	defer res.Body.Close()
	var out struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
	}
	json.NewDecoder(res.Body).Decode(&out)
	if !out.OK {
		return fmt.Errorf("Telegram refused the message (%d %s)", res.StatusCode, out.Description)
	}
	return nil
}

// adminSchema is created with the other tables (openDB).
const adminSchema = `
CREATE TABLE IF NOT EXISTS admin_codes (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  salt TEXT NOT NULL,
  code_hash TEXT NOT NULL,       -- sha256(salt + code): the code itself is never stored
  created_at INTEGER NOT NULL,
  expires_at INTEGER NOT NULL,
  tries INTEGER NOT NULL DEFAULT 0,
  done INTEGER NOT NULL DEFAULT 0, -- used, voided, or never sent
  ip TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS admin_sessions (
  token_hash TEXT PRIMARY KEY,   -- sha256 of the cookie
  created_at INTEGER NOT NULL,
  expires_at INTEGER NOT NULL,
  last_seen INTEGER NOT NULL,
  ip TEXT NOT NULL DEFAULT '',
  agent TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS deleted_games (
  code TEXT PRIMARY KEY,
  host_secret TEXT NOT NULL,     -- so the Keeper's Stories page still opens
  deleted_at INTEGER NOT NULL,
  summary TEXT NOT NULL          -- JSON: how the game stood when it was deleted
);`

// adminMu keeps two code requests (or a request and a check) from racing.
var adminMu sync.Mutex

func sha(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func randToken(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// clientIP is who is asking: behind nginx (and Cloudflare) the proxy's
// headers say; otherwise the connection does. It is only shown to El.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		for _, h := range []string{"CF-Connecting-IP", "X-Real-IP"} {
			if v := strings.TrimSpace(r.Header.Get(h)); v != "" {
				return v
			}
		}
		if v := r.Header.Get("X-Forwarded-For"); v != "" {
			return strings.TrimSpace(strings.Split(v, ",")[0])
		}
	}
	return host
}

// isAdmin says whether the request carries a live admin session.
func (s *Server) isAdmin(r *http.Request) bool {
	c, err := r.Cookie(adminCookie)
	if err != nil || c.Value == "" {
		return false
	}
	h, now := sha(c.Value), time.Now().Unix()
	var expires, seen int64
	if err := s.db.QueryRow(`SELECT expires_at, last_seen FROM admin_sessions WHERE token_hash=?`, h).Scan(&expires, &seen); err != nil || expires <= now {
		return false
	}
	if now-seen > 60 {
		s.db.Exec(`UPDATE admin_sessions SET last_seen=? WHERE token_hash=?`, now, h)
	}
	return true
}

// adminOnly wraps every admin route that isn't part of logging in.
func (s *Server) adminOnly(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.isAdmin(r) {
			fail(w, 401, "log in first")
			return
		}
		h(w, r)
	}
}

// codeWait is how long until a new code may be sent: 30 s after the last one,
// and an hour after the oldest of the last 6 sent within the hour.
func (s *Server) codeWait(now time.Time) time.Duration {
	var last, sixth int64
	s.db.QueryRow(`SELECT COALESCE(MAX(created_at), 0) FROM admin_codes`).Scan(&last)
	wait := time.Unix(last, 0).Add(adminCodeGap).Sub(now)
	if s.db.QueryRow(`SELECT created_at FROM admin_codes WHERE created_at>? ORDER BY created_at DESC LIMIT 1 OFFSET ?`,
		now.Add(-time.Hour).Unix(), adminCodesHour-1).Scan(&sixth) == nil {
		wait = max(wait, time.Unix(sixth, 0).Add(time.Hour).Sub(now))
	}
	return max(wait, 0)
}

// adminMe tells the page whether Telegram is set up and whether it's logged in.
//
// GET /api/admin/me → {configured, loggedIn, wait}
func (s *Server) adminMe(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"configured": tg.ready(), "loggedIn": s.isAdmin(r),
		"wait": int(s.codeWait(time.Now()).Seconds() + .999)})
}

// adminCode sends El a new login code.
//
// POST /api/admin/code → {ok, expires} | 429 {error, wait} | 503 (Telegram not set up) | 502 (Telegram failed)
func (s *Server) adminCode(w http.ResponseWriter, r *http.Request) {
	if !tg.ready() {
		fail(w, 503, errTelegramOff.Error())
		return
	}
	adminMu.Lock()
	defer adminMu.Unlock()
	now := time.Now()
	if wait := s.codeWait(now); wait > 0 {
		secs := int(wait.Seconds() + .999)
		msg := fmt.Sprintf("A code was sent a moment ago. Wait %d s, then try again.", secs)
		if secs > 60 {
			msg = fmt.Sprintf("That's %d codes this hour, the most there can be. Try again in %d min.", adminCodesHour, (secs+59)/60)
		}
		writeJSON(w, 429, map[string]any{"error": msg, "wait": secs})
		return
	}
	n, _ := rand.Int(rand.Reader, big.NewInt(1000000))
	code, salt, ip := fmt.Sprintf("%06d", n), randToken(12), clientIP(r)
	res, err := s.db.Exec(`INSERT INTO admin_codes(salt, code_hash, created_at, expires_at, ip) VALUES(?,?,?,?,?)`,
		salt, sha(salt+code), now.Unix(), now.Add(adminCodeLife).Unix(), ip)
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	id, _ := res.LastInsertId()
	msg := fmt.Sprintf("🔐 Heartwood login code <code>%s</code>\n• from %s\n• expires in 5 min", code, html.EscapeString(ip))
	if err := tg.send(msg); err != nil {
		s.db.Exec(`UPDATE admin_codes SET done=1 WHERE id=?`, id)
		log.Printf("admin: login code not sent: %v", err)
		fail(w, 502, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "expires": now.Add(adminCodeLife).Unix()})
}

// adminLogin checks a code against every code still waiting; the right one
// gives this browser a session. Each wrong try counts against all of them.
//
// POST /api/admin/login {code} → {ok} + the session cookie | 400 {error}
func (s *Server) adminLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code string `json:"code"`
	}
	json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&req)
	code := strings.Map(func(c rune) rune {
		if c >= '0' && c <= '9' {
			return c
		}
		return -1
	}, req.Code)
	adminMu.Lock()
	defer adminMu.Unlock()
	now := time.Now().Unix()
	type pending struct {
		id         int64
		salt, hash string
	}
	var waiting []pending
	rows, err := s.db.Query(`SELECT id, salt, code_hash FROM admin_codes WHERE done=0 AND expires_at>? AND tries<?`, now, adminCodeTries)
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	for rows.Next() {
		var p pending
		rows.Scan(&p.id, &p.salt, &p.hash)
		waiting = append(waiting, p)
	}
	rows.Close()
	if len(waiting) == 0 {
		fail(w, 400, "That code has expired, or was used. Send a new one.")
		return
	}
	match := int64(0)
	if len(code) == 6 {
		for _, p := range waiting {
			if subtle.ConstantTimeCompare([]byte(sha(p.salt+code)), []byte(p.hash)) == 1 {
				match = p.id
			}
		}
	}
	if match == 0 {
		s.db.Exec(`UPDATE admin_codes SET tries=tries+1 WHERE done=0 AND expires_at>?`, now)
		s.db.Exec(`UPDATE admin_codes SET done=1 WHERE done=0 AND tries>=?`, adminCodeTries)
		var left int
		s.db.QueryRow(`SELECT COUNT(*) FROM admin_codes WHERE done=0 AND expires_at>?`, now).Scan(&left)
		if left == 0 {
			fail(w, 400, "Too many wrong tries: that code is void. Send a new one.")
			return
		}
		fail(w, 400, "That code didn't work. Check it and try again.")
		return
	}
	s.db.Exec(`UPDATE admin_codes SET done=1 WHERE id=?`, match)
	ip := clientIP(r)
	var seen int
	s.db.QueryRow(`SELECT COUNT(*) FROM admin_sessions WHERE ip=?`, ip).Scan(&seen)
	token := randToken(32)
	agent := r.UserAgent()
	if len(agent) > 200 {
		agent = agent[:200]
	}
	if _, err := s.db.Exec(`INSERT INTO admin_sessions(token_hash, created_at, expires_at, last_seen, ip, agent) VALUES(?,?,?,?,?,?)`,
		sha(token), now, now+int64(adminSessionLife.Seconds()), now, ip, agent); err != nil {
		fail(w, 500, err.Error())
		return
	}
	http.SetCookie(w, &http.Cookie{Name: adminCookie, Value: token, Path: "/", MaxAge: int(adminSessionLife.Seconds()),
		HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode})
	if seen == 0 {
		go func() {
			msg := fmt.Sprintf("⚠️ Heartwood admin login from a new place\n• %s\n• not you? Log out everywhere on the admin page", html.EscapeString(ip))
			if err := tg.send(msg); err != nil {
				log.Printf("admin: new-place alert not sent: %v", err)
			}
		}()
	}
	log.Printf("admin: logged in from %s", ip)
	writeJSON(w, 200, map[string]any{"ok": true})
}

// adminLogout ends this browser's session, or every session ({all: true}).
// It needs a session itself, so nobody else can log El out everywhere.
//
// POST /api/admin/logout {all} → {ok}
func (s *Server) adminLogout(w http.ResponseWriter, r *http.Request) {
	var req struct {
		All bool `json:"all"`
	}
	json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&req)
	if req.All {
		s.db.Exec(`DELETE FROM admin_sessions`)
	} else if c, err := r.Cookie(adminCookie); err == nil {
		s.db.Exec(`DELETE FROM admin_sessions WHERE token_hash=?`, sha(c.Value))
	}
	http.SetCookie(w, &http.Cookie{Name: adminCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode})
	writeJSON(w, 200, map[string]any{"ok": true})
}

// sweepAdmin forgets old codes and expired sessions (with the photo sweep).
func (s *Server) sweepAdmin(now time.Time) {
	s.db.Exec(`DELETE FROM admin_codes WHERE created_at<?`, now.Add(-24*time.Hour).Unix())
	s.db.Exec(`DELETE FROM admin_sessions WHERE expires_at<?`, now.Unix())
}

// ---- games ----

// AdminPlayer is a player as the admin shows them.
type AdminPlayer struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Color    string `json:"color"`
	Photo    int    `json:"photo"`
	Bot      bool   `json:"bot"`
	Types    []int  `json:"types"`
	Value    string `json:"value"` // the value they stand for, "" before entering
	Water    int    `json:"water"`
	Fruit    int    `json:"fruit"`
	Placed   int    `json:"placed"`
	Treasure string `json:"treasure"`
}

// AdminGame is one row of the list, and the head of a game's page.
type AdminGame struct {
	Code      string        `json:"code"`
	Phase     string        `json:"phase"` // lobby … end, or "deleted" (stories only)
	Round     int           `json:"round"`
	Tide      int           `json:"tide"`
	Result    string        `json:"result"`
	Live      bool          `json:"live"`
	Players   []AdminPlayer `json:"players"`
	CreatedAt int64         `json:"createdAt"` // 0: made before games noted it
	UpdatedAt int64         `json:"updatedAt"`
	EndedAt   int64         `json:"endedAt"`
	DeletedAt int64         `json:"deletedAt,omitempty"`
	Recording bool          `json:"recording"`
	Stories   int           `json:"stories"`
	Photos    int           `json:"photos"`
}

// AdminDetail is a game's page: the row, its progress, links and what can be done now.
type AdminDetail struct {
	AdminGame
	Goals      *Goals `json:"goals,omitempty"`
	Sharing    bool   `json:"sharing"`
	MinPlayers int    `json:"minPlayers"`
	MaxPlayers int    `json:"maxPlayers"`
	KeeperLink string `json:"keeperLink,omitempty"`
	StoriesURL string `json:"storiesUrl"`
	CanBot     bool   `json:"canBot"`
	CanKick    bool   `json:"canKick"`
	CanEnd     bool   `json:"canEnd"`
	EndWhy     string `json:"endWhy,omitempty"`
	TypeCode   bool   `json:"typeCode"` // deleting asks for the code to be typed
}

var botName = regexp.MustCompile(`^Bot \d+$`)

func adminPlayers(g *Game) []AdminPlayer {
	out := []AdminPlayer{}
	for _, p := range g.Players {
		ap := AdminPlayer{ID: p.ID, Name: p.Name, Color: p.Color, Photo: p.Photo, Bot: botName.MatchString(p.Name),
			Types: append([]int{}, p.Types...), Water: p.Water, Fruit: p.Fruit, Placed: p.Placed, Treasure: p.Treasure}
		if p.Value >= 0 && p.Value < len(Values) {
			ap.Value = Values[p.Value].Name
		}
		out = append(out, ap)
	}
	return out
}

// livePhase: started and not over (briefing, play and the finale). Deleting one asks for its code.
func livePhase(phase string) bool {
	return phase != PhaseLobby && phase != PhaseEnd
}

// counts reads one number per game code: stories, photos, last save.
func (s *Server) counts(query string, args ...any) map[string]int64 {
	out := map[string]int64{}
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var code string
		var n int64
		if rows.Scan(&code, &n) == nil {
			out[code] = n
		}
	}
	return out
}

func (s *Server) adminRow(g *Game, stories, photos, updated map[string]int64) AdminGame {
	return AdminGame{Code: g.Code, Phase: g.Phase, Round: g.Round, Tide: g.Tide, Result: g.Result, Live: g.Phase != PhaseEnd,
		Players: adminPlayers(g), CreatedAt: g.CreatedAt, UpdatedAt: updated[g.Code], EndedAt: g.EndedAt, Recording: !g.NoRecord,
		Stories: int(stories[g.Code]), Photos: int(photos[g.Code])}
}

// deletedRows are the games deleted while they had stories.
func (s *Server) deletedRows(stories map[string]int64) []AdminGame {
	out := []AdminGame{}
	rows, err := s.db.Query(`SELECT code, deleted_at, summary FROM deleted_games`)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var a AdminGame
		var sum string
		if rows.Scan(&a.Code, &a.DeletedAt, &sum) != nil {
			continue
		}
		json.Unmarshal([]byte(sum), &a)
		a.Phase, a.Live, a.UpdatedAt, a.Stories, a.Photos, a.Recording = "deleted", false, a.DeletedAt, int(stories[a.Code]), 0, false
		if a.Players == nil {
			a.Players = []AdminPlayer{}
		}
		out = append(out, a)
	}
	return out
}

// adminGames lists every game, newest activity first.
//
// GET /api/admin/games → {games: [AdminGame], now}
func (s *Server) adminGames(w http.ResponseWriter, r *http.Request) {
	stories := s.counts(`SELECT code, COUNT(*) FROM recordings GROUP BY code`)
	photos := s.counts(`SELECT code, COUNT(*) FROM photos GROUP BY code`)
	updated := s.counts(`SELECT code, updated_at FROM games`)
	s.mu.Lock()
	out := []AdminGame{}
	for _, g := range s.games {
		out = append(out, s.adminRow(g, stories, photos, updated))
	}
	s.mu.Unlock()
	out = append(out, s.deletedRows(stories)...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].UpdatedAt != out[j].UpdatedAt {
			return out[i].UpdatedAt > out[j].UpdatedAt
		}
		return out[i].Code < out[j].Code
	})
	writeJSON(w, 200, map[string]any{"games": out, "now": time.Now().Unix()})
}

// adminDetail is one game's page.
//
// GET /api/admin/games/{code} → AdminDetail
func (s *Server) adminDetail(w http.ResponseWriter, r *http.Request) {
	code := strings.ToUpper(r.PathValue("code"))
	stories := s.counts(`SELECT code, COUNT(*) FROM recordings WHERE code=? GROUP BY code`, code)
	photos := s.counts(`SELECT code, COUNT(*) FROM photos WHERE code=? GROUP BY code`, code)
	updated := s.counts(`SELECT code, updated_at FROM games WHERE code=?`, code)
	s.mu.Lock()
	g := s.games[code]
	if g == nil {
		s.mu.Unlock()
		for _, a := range s.deletedRows(stories) {
			if a.Code == code {
				var secret string
				s.db.QueryRow(`SELECT host_secret FROM deleted_games WHERE code=?`, code).Scan(&secret)
				writeJSON(w, 200, AdminDetail{AdminGame: a, StoriesURL: storiesURL(code, secret), TypeCode: true,
					MinPlayers: MinPlayers, MaxPlayers: MaxPlayers})
				return
			}
		}
		fail(w, 404, "no game with that code")
		return
	}
	d := AdminDetail{AdminGame: s.adminRow(g, stories, photos, updated), Sharing: len(g.Shares) > 0,
		MinPlayers: MinPlayers, MaxPlayers: MaxPlayers,
		KeeperLink: "/board.html?g=" + url.QueryEscape(g.Code) + "&k=" + url.QueryEscape(g.HostSecret),
		StoriesURL: storiesURL(g.Code, g.HostSecret),
		CanBot:     g.Phase == PhaseLobby && len(g.Players) < MaxPlayers,
		CanKick:    g.Phase == PhaseLobby, TypeCode: livePhase(g.Phase)}
	if g.Phase != PhaseLobby {
		goals := g.goals()
		d.Goals = &goals
	}
	switch {
	case g.Phase != PhaseTurn:
		d.EndWhy = "Only during play."
	case len(g.Shares) > 0:
		d.EndWhy = "Someone is sharing: finish the share first."
	default:
		d.CanEnd = true
	}
	s.mu.Unlock()
	writeJSON(w, 200, d)
}

func storiesURL(code, secret string) string {
	return "/stories.html?g=" + url.QueryEscape(code) + "&k=" + url.QueryEscape(secret)
}

// adminGame finds a live game for an action; the caller holds s.mu.
func (s *Server) adminGame(w http.ResponseWriter, r *http.Request) *Game {
	g := s.games[strings.ToUpper(r.PathValue("code"))]
	if g == nil {
		fail(w, 404, "no game with that code")
	}
	return g
}

// adminDid logs an admin action and tells every screen of the game.
func (s *Server) adminDid(w http.ResponseWriter, g *Game, typ string, payload map[string]any) {
	b, _ := json.Marshal(payload)
	s.db.Exec(`INSERT INTO events(code,actor,type,payload,at) VALUES(?,?,?,?,?)`, g.Code, "admin", typ, string(b), time.Now().Unix())
	if err := s.changed(g); err != nil {
		fail(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

// adminBot adds a bot seat: the next free "Bot N" and the next free colour.
// It is an ordinary seat; the Keeper taps its moves.
//
// POST /api/admin/games/{code}/bot → {ok, name, color}
func (s *Server) adminBot(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	g := s.adminGame(w, r)
	if g == nil {
		return
	}
	names, colors := map[string]bool{}, map[string]bool{}
	for _, p := range g.Players {
		names[p.Name], colors[p.Color] = true, true
	}
	name := ""
	for n := 1; name == ""; n++ {
		if !names[fmt.Sprintf("Bot %d", n)] {
			name = fmt.Sprintf("Bot %d", n)
		}
	}
	color := ""
	for _, c := range Colors {
		if !colors[c] {
			color = c
			break
		}
	}
	if color == "" { // only when players brought colours of their own
		color = "#8d6e63"
	}
	p, err := g.Join(name, color)
	if err != nil {
		fail(w, 400, err.Error())
		return
	}
	b, _ := json.Marshal(map[string]any{"pid": p.ID, "name": name, "color": color})
	s.db.Exec(`INSERT INTO events(code,actor,type,payload,at) VALUES(?,?,?,?,?)`, g.Code, "admin", "bot", string(b), time.Now().Unix())
	if err := s.changed(g); err != nil {
		fail(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "name": name, "color": color})
}

// adminKick removes a player in the lobby (the Keeper's kick), with their photo.
//
// POST /api/admin/games/{code}/kick {pid} → {ok}
func (s *Server) adminKick(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Pid string `json:"pid"`
	}
	json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&req)
	s.mu.Lock()
	defer s.mu.Unlock()
	g := s.adminGame(w, r)
	if g == nil {
		return
	}
	if err := g.Apply(Action{Host: g.HostSecret, Type: "kick", Target: req.Pid, Hex: -1}); err != nil {
		fail(w, 400, err.Error())
		return
	}
	s.deletePhoto(g.Code, req.Pid)
	s.adminDid(w, g, "kick", map[string]any{"target": req.Pid})
}

// adminEnd closes a game early (the Keeper's End the game early).
//
// POST /api/admin/games/{code}/end → {ok}
func (s *Server) adminEnd(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	g := s.adminGame(w, r)
	if g == nil {
		return
	}
	if err := g.Apply(Action{Host: g.HostSecret, Type: "endGame", Hex: -1}); err != nil {
		fail(w, 400, err.Error())
		return
	}
	s.adminDid(w, g, "endGame", nil)
}

// adminDelete deletes a game and its photos. Its stories stay (the team's
// record): the game then shows as "stories only". A game still being played
// needs its code typed ({confirm: "C7W7"}).
//
// DELETE /api/admin/games/{code} {confirm} → {ok, storiesKept}
func (s *Server) adminDelete(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Confirm string `json:"confirm"`
	}
	json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&req)
	s.mu.Lock()
	defer s.mu.Unlock()
	g := s.adminGame(w, r)
	if g == nil {
		return
	}
	if livePhase(g.Phase) && !strings.EqualFold(strings.TrimSpace(req.Confirm), g.Code) {
		fail(w, 400, fmt.Sprintf("%s is still being played: type its code to delete it", g.Code))
		return
	}
	var stories int
	s.db.QueryRow(`SELECT COUNT(*) FROM recordings WHERE code=?`, g.Code).Scan(&stories)
	now := time.Now().Unix()
	if stories > 0 {
		sum, _ := json.Marshal(map[string]any{"round": g.Round, "tide": g.Tide, "result": g.Result, "createdAt": g.CreatedAt,
			"endedAt": g.EndedAt, "players": adminPlayers(g), "deletedPhase": g.Phase})
		if _, err := s.db.Exec(`INSERT INTO deleted_games(code, host_secret, deleted_at, summary) VALUES(?,?,?,?)
ON CONFLICT(code) DO UPDATE SET host_secret=excluded.host_secret, deleted_at=excluded.deleted_at, summary=excluded.summary`,
			g.Code, g.HostSecret, now, string(sum)); err != nil {
			fail(w, 500, err.Error())
			return
		}
	}
	for _, q := range []string{`DELETE FROM games WHERE code=?`, `DELETE FROM photos WHERE code=?`, `DELETE FROM events WHERE code=?`} {
		if _, err := s.db.Exec(q, g.Code); err != nil {
			fail(w, 500, err.Error())
			return
		}
	}
	delete(s.games, g.Code)
	s.db.Exec(`INSERT INTO events(code,actor,type,payload,at) VALUES(?,?,?,?,?)`, g.Code, "admin", "deleteGame",
		fmt.Sprintf(`{"phase":%q,"players":%d,"stories":%d}`, g.Phase, len(g.Players), stories), now)
	// screens still open on it hear that it's gone
	for ch := range s.subs[g.Code] {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
	log.Printf("admin: deleted game %s (%s, %d players, %d stories kept)", g.Code, g.Phase, len(g.Players), stories)
	writeJSON(w, 200, map[string]any{"ok": true, "storiesKept": stories})
}

// adminDeleteStories deletes the stories a deleted game left, for good.
//
// DELETE /api/admin/games/{code}/stories {confirm} → {ok, deleted}
func (s *Server) adminDeleteStories(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Confirm string `json:"confirm"`
	}
	json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&req)
	code := strings.ToUpper(r.PathValue("code"))
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.games[code] != nil {
		fail(w, 400, "delete the game first; its stories go only after that")
		return
	}
	var n int
	if s.db.QueryRow(`SELECT COUNT(*) FROM deleted_games WHERE code=?`, code).Scan(&n); n == 0 {
		fail(w, 404, "no deleted game with that code")
		return
	}
	if !strings.EqualFold(strings.TrimSpace(req.Confirm), code) {
		fail(w, 400, "type the game's code to delete its stories")
		return
	}
	res, err := s.db.Exec(`DELETE FROM recordings WHERE code=?`, code)
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	gone, _ := res.RowsAffected()
	s.db.Exec(`DELETE FROM deleted_games WHERE code=?`, code)
	s.db.Exec(`INSERT INTO events(code,actor,type,payload,at) VALUES(?,?,?,?,?)`, code, "admin", "deleteStories", fmt.Sprintf(`{"stories":%d}`, gone), time.Now().Unix())
	log.Printf("admin: deleted the %d stories of game %s", gone, code)
	writeJSON(w, 200, map[string]any{"ok": true, "deleted": gone})
}

// codeTaken: a new game must not reuse the code of one that was deleted or
// whose stories are still kept (their recordings are filed by code).
func (s *Server) codeTaken(code string) bool {
	var n int
	s.db.QueryRow(`SELECT (SELECT COUNT(*) FROM deleted_games WHERE code=?) + (SELECT COUNT(*) FROM recordings WHERE code=?) + (SELECT COUNT(*) FROM games WHERE code=?)`,
		code, code, code).Scan(&n)
	return n > 0
}

// deletedSecret is the Keeper secret a deleted game left with its stories.
func (s *Server) deletedSecret(code string) (string, bool) {
	var secret string
	err := s.db.QueryRow(`SELECT host_secret FROM deleted_games WHERE code=?`, code).Scan(&secret)
	return secret, err == nil
}

// serveAdmin serves the admin page at /admin, never cached and never framed.
func serveAdmin(w http.ResponseWriter, r *http.Request) {
	b, err := webFS.ReadFile("web/admin.html")
	if err != nil {
		http.Error(w, "no admin page", 500)
		return
	}
	adminHeaders(w)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(b)
}

func adminHeaders(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Referrer-Policy", "no-referrer")
}

func (s *Server) adminRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /admin", serveAdmin)
	mux.HandleFunc("GET /admin.html", serveAdmin)
	// logging in: no session needed
	mux.HandleFunc("GET /api/admin/me", s.adminMe)
	mux.HandleFunc("POST /api/admin/code", s.adminCode)
	mux.HandleFunc("POST /api/admin/login", s.adminLogin)
	// everything else: a session
	for pattern, h := range adminSessionRoutes(s) {
		mux.HandleFunc(pattern, s.adminOnly(h))
	}
}

// adminSessionRoutes are the routes that need a session (the tests walk them all).
func adminSessionRoutes(s *Server) map[string]http.HandlerFunc {
	return map[string]http.HandlerFunc{
		"POST /api/admin/logout":                 s.adminLogout,
		"GET /api/admin/games":                   s.adminGames,
		"GET /api/admin/games/{code}":            s.adminDetail,
		"POST /api/admin/games/{code}/bot":       s.adminBot,
		"POST /api/admin/games/{code}/kick":      s.adminKick,
		"POST /api/admin/games/{code}/end":       s.adminEnd,
		"DELETE /api/admin/games/{code}":         s.adminDelete,
		"DELETE /api/admin/games/{code}/stories": s.adminDeleteStories,
	}
}
