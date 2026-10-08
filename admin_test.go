package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeTelegram is a stand-in Bot API: it keeps what was sent to it.
type fakeTelegram struct {
	mu   sync.Mutex
	msgs []map[string]any
	path []string
	ok   bool
}

func (f *fakeTelegram) sent() []map[string]any {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]map[string]any(nil), f.msgs...)
}

var codeIn = regexp.MustCompile(`<code>(\d{6})</code>`)

// lastCode is the code in the newest message.
func (f *fakeTelegram) lastCode(t *testing.T) string {
	t.Helper()
	m := f.sent()
	if len(m) == 0 {
		t.Fatal("nothing sent to Telegram")
	}
	c := codeIn.FindStringSubmatch(m[len(m)-1]["text"].(string))
	if c == nil {
		t.Fatalf("no code in %q", m[len(m)-1]["text"])
	}
	return c[1]
}

// withTelegram points the admin at a fake Bot API with a token and chat set.
func withTelegram(t *testing.T) *fakeTelegram {
	t.Helper()
	f := &fakeTelegram{ok: true}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var m map[string]any
		json.NewDecoder(r.Body).Decode(&m)
		f.mu.Lock()
		f.msgs, f.path = append(f.msgs, m), append(f.path, r.URL.Path)
		ok := f.ok
		f.mu.Unlock()
		if !ok {
			w.WriteHeader(400)
			io.WriteString(w, `{"ok":false,"description":"Bad Request: chat not found"}`)
			return
		}
		io.WriteString(w, `{"ok":true,"result":{}}`)
	}))
	t.Cleanup(srv.Close)
	old := tg
	t.Cleanup(func() { tg = old })
	tg = telegramConfig{TokenVar: "HW_TEST_TG_TOKEN", ChatVar: "HW_TEST_TG_CHAT", API: srv.URL}
	t.Setenv("HW_TEST_TG_TOKEN", "123:secret-token")
	t.Setenv("HW_TEST_TG_CHAT", "4242")
	return f
}

// adm sends one request with an optional admin cookie.
func (ps *photoServer) adm(method, path string, body any, cookie *http.Cookie) (int, map[string]any, *httptest.ResponseRecorder) {
	ps.t.Helper()
	b, _ := json.Marshal(body)
	r := httptest.NewRequest(method, path, bytes.NewReader(b))
	r.RemoteAddr = "203.0.113.7:5555"
	if cookie != nil {
		r.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	ps.h.ServeHTTP(rec, r)
	var out map[string]any
	json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out, rec
}

// allowNextCode lets a new code go out at once (instead of waiting 30 s).
func (ps *photoServer) allowNextCode() {
	ps.s.db.Exec(`UPDATE admin_codes SET created_at=created_at-3700`) // past the 30 s gap and the hourly cap
}

// login runs the whole login and returns the session cookie.
func (ps *photoServer) login(f *fakeTelegram) *http.Cookie {
	ps.t.Helper()
	ps.allowNextCode()
	if c, out, _ := ps.adm("POST", "/api/admin/code", nil, nil); c != 200 {
		ps.t.Fatalf("code: %d %v", c, out)
	}
	c, out, rec := ps.adm("POST", "/api/admin/login", map[string]string{"code": f.lastCode(ps.t)}, nil)
	if c != 200 {
		ps.t.Fatalf("login: %d %v", c, out)
	}
	for _, ck := range rec.Result().Cookies() {
		if ck.Name == adminCookie {
			return ck
		}
	}
	ps.t.Fatal("no session cookie")
	return nil
}

func TestAdminLogin(t *testing.T) {
	f := withTelegram(t)
	ps := newPhotoServer(t)

	_, me, _ := ps.adm("GET", "/api/admin/me", nil, nil)
	if me["configured"] != true || me["loggedIn"] != false {
		t.Fatalf("me before: %v", me)
	}
	if c, out, _ := ps.adm("POST", "/api/admin/code", nil, nil); c != 200 || out["ok"] != true {
		t.Fatalf("code: %d %v", c, out)
	}
	code := f.lastCode(t)
	if f.path[0] != "/bot123:secret-token/sendMessage" || f.sent()[0]["chat_id"] != "4242" ||
		!strings.Contains(f.sent()[0]["text"].(string), "from 203.0.113.7") {
		t.Fatalf("telegram got %v %v", f.path, f.sent())
	}
	// only the hash is kept
	var salt, hash string
	ps.s.db.QueryRow(`SELECT salt, code_hash FROM admin_codes`).Scan(&salt, &hash)
	if hash == "" || strings.Contains(hash, code) || hash != sha(salt+code) {
		t.Fatalf("stored %q", hash)
	}
	// one code per 30 s
	if c, out, _ := ps.adm("POST", "/api/admin/code", nil, nil); c != 429 || out["wait"] == nil {
		t.Fatalf("second code at once: %d %v", c, out)
	}
	// wrong tries: 5 void it, and then even the right code fails
	wrong := "000000"
	if code == wrong {
		wrong = "111111"
	}
	for i := 1; i <= adminCodeTries; i++ {
		c, out, _ := ps.adm("POST", "/api/admin/login", map[string]string{"code": wrong}, nil)
		if c != 400 || (i == adminCodeTries) != strings.Contains(out["error"].(string), "void") {
			t.Fatalf("wrong try %d: %d %v", i, c, out)
		}
	}
	if c, _, _ := ps.adm("POST", "/api/admin/login", map[string]string{"code": code}, nil); c != 400 {
		t.Fatal("a voided code still works")
	}
	// an expired code fails
	ps.allowNextCode()
	ps.adm("POST", "/api/admin/code", nil, nil)
	code = f.lastCode(t)
	ps.s.db.Exec(`UPDATE admin_codes SET expires_at=?`, time.Now().Add(-time.Second).Unix())
	if c, _, _ := ps.adm("POST", "/api/admin/login", map[string]string{"code": code}, nil); c != 400 {
		t.Fatal("an expired code works")
	}
	// the right code, typed with a space, works once
	ps.allowNextCode()
	ps.adm("POST", "/api/admin/code", nil, nil)
	code = f.lastCode(t)
	c, out, rec := ps.adm("POST", "/api/admin/login", map[string]string{"code": code[:3] + " " + code[3:]}, nil)
	if c != 200 || out["ok"] != true {
		t.Fatalf("login: %d %v", c, out)
	}
	if c, _, _ := ps.adm("POST", "/api/admin/login", map[string]string{"code": code}, nil); c != 400 {
		t.Fatal("a code worked twice")
	}
	var ck *http.Cookie
	for _, x := range rec.Result().Cookies() {
		if x.Name == adminCookie {
			ck = x
		}
	}
	if ck == nil || !ck.HttpOnly || !ck.Secure || ck.SameSite != http.SameSiteStrictMode || ck.MaxAge != 30*24*3600 || ck.Path != "/" {
		t.Fatalf("cookie %+v", ck)
	}
	var stored string
	var expires int64
	ps.s.db.QueryRow(`SELECT token_hash, expires_at FROM admin_sessions`).Scan(&stored, &expires)
	if stored != sha(ck.Value) || stored == ck.Value || expires < time.Now().Add(29*24*time.Hour).Unix() {
		t.Fatalf("session stored %q expires %d", stored, expires)
	}
	if _, me, _ := ps.adm("GET", "/api/admin/me", nil, ck); me["loggedIn"] != true {
		t.Fatalf("me after: %v", me)
	}
	// a first login from this place sends an alert
	deadline := time.Now().Add(2 * time.Second)
	for !strings.Contains(f.sent()[len(f.sent())-1]["text"].(string), "new place") {
		if time.Now().After(deadline) {
			t.Fatal("no new-place alert")
		}
		time.Sleep(10 * time.Millisecond)
	}
	// logging out ends it
	if c, _, _ := ps.adm("POST", "/api/admin/logout", nil, ck); c != 200 {
		t.Fatal("logout")
	}
	if c, _, _ := ps.adm("GET", "/api/admin/games", nil, ck); c != 401 {
		t.Fatal("still in after logout")
	}
	// log out everywhere
	a, b := ps.login(f), ps.login(f)
	if c, _, _ := ps.adm("POST", "/api/admin/logout", map[string]bool{"all": true}, a); c != 200 {
		t.Fatal("logout all")
	}
	if c, _, _ := ps.adm("GET", "/api/admin/games", nil, b); c != 401 {
		t.Fatal("the other browser is still in")
	}
	// an expired session is out
	d := ps.login(f)
	ps.s.db.Exec(`UPDATE admin_sessions SET expires_at=?`, time.Now().Add(-time.Second).Unix())
	if c, _, _ := ps.adm("GET", "/api/admin/games", nil, d); c != 401 {
		t.Fatal("expired session still in")
	}
}

// At most 6 codes an hour, however long the wait between them.
func TestAdminCodesPerHour(t *testing.T) {
	withTelegram(t)
	ps := newPhotoServer(t)
	now := time.Now()
	for k := range adminCodesHour {
		at := now.Add(-time.Duration(50-k*5) * time.Minute).Unix() // 50, 45 … 25 minutes ago
		ps.s.db.Exec(`INSERT INTO admin_codes(salt, code_hash, created_at, expires_at, done) VALUES('s','h',?,?,1)`, at, at+300)
	}
	c, out, _ := ps.adm("POST", "/api/admin/code", nil, nil)
	if wait, _ := out["wait"].(float64); c != 429 || wait < 9*60 || wait > 10*60+1 || !strings.Contains(out["error"].(string), "this hour") {
		t.Fatalf("7th code in an hour: %d %v", c, out)
	}
	ps.s.db.Exec(`UPDATE admin_codes SET created_at=created_at-600`) // the oldest is now an hour old
	if c, out, _ := ps.adm("POST", "/api/admin/code", nil, nil); c != 200 {
		t.Fatalf("after the hour: %d %v", c, out)
	}
}

func TestAdminRoutesNeedSession(t *testing.T) {
	f := withTelegram(t)
	ps := newPhotoServer(t)
	ps.add(newTable(t, 6, nil), "ABCD")
	bogus := &http.Cookie{Name: adminCookie, Value: "made-up"}
	for pattern := range adminSessionRoutes(ps.s) {
		method, path, _ := strings.Cut(pattern, " ")
		path = strings.ReplaceAll(path, "{code}", "ABCD")
		for _, ck := range []*http.Cookie{nil, bogus} {
			if c, _, _ := ps.adm(method, path, map[string]any{"confirm": "ABCD", "all": true}, ck); c != 401 {
				t.Fatalf("%s without a session: %d", pattern, c)
			}
		}
	}
	if ps.s.games["ABCD"] == nil {
		t.Fatal("the game went without a session")
	}
	// with one, the list opens
	if c, out, _ := ps.adm("GET", "/api/admin/games", nil, ps.login(f)); c != 200 || len(out["games"].([]any)) != 1 {
		t.Fatalf("list with a session: %d %v", c, out)
	}
}

func TestAdminTelegramOff(t *testing.T) {
	old := tg
	t.Cleanup(func() { tg = old })
	tg = telegramConfig{TokenVar: "HW_TEST_NO_TOKEN", ChatVar: "HW_TEST_NO_CHAT", API: "http://127.0.0.1:1"}
	ps := newPhotoServer(t)
	if _, me, _ := ps.adm("GET", "/api/admin/me", nil, nil); me["configured"] != false || me["loggedIn"] != false {
		t.Fatalf("me: %v", me)
	}
	if c, out, _ := ps.adm("POST", "/api/admin/code", nil, nil); c != 503 || !strings.Contains(out["error"].(string), "isn't set up") {
		t.Fatalf("code: %d %v", c, out)
	}
	// the game works as before
	if c, out, _ := ps.adm("POST", "/api/games", nil, nil); c != 200 || out["code"] == nil {
		t.Fatalf("create: %d %v", c, out)
	}
}

func TestAdminTelegramErrorsHideTheToken(t *testing.T) {
	f := withTelegram(t)
	ps := newPhotoServer(t)
	f.ok = false
	c, out, _ := ps.adm("POST", "/api/admin/code", nil, nil)
	if c != 502 || strings.Contains(out["error"].(string), "secret-token") || !strings.Contains(out["error"].(string), "chat not found") {
		t.Fatalf("refused: %d %v", c, out)
	}
	// unreachable: a url.Error would print the URL with the token
	tg.API = "http://127.0.0.1:1"
	ps.allowNextCode()
	c, out, _ = ps.adm("POST", "/api/admin/code", nil, nil)
	if c != 502 || strings.Contains(out["error"].(string), "secret-token") {
		t.Fatalf("unreachable: %d %v", c, out)
	}
	// a code that never went out can't be used
	var done int
	ps.s.db.QueryRow(`SELECT MIN(done) FROM admin_codes`).Scan(&done)
	if done != 1 {
		t.Fatal("an unsent code is still waiting")
	}
}

func TestAdminGamesAndActions(t *testing.T) {
	f := withTelegram(t)
	ps := newPhotoServer(t)
	ck := ps.login(f)

	lobby := ps.add(newTable(t, 3, nil), "LOBY")
	play := newTable(t, 6, nil)
	play.play()
	ps.add(play, "PLAY")
	ps.s.db.Exec(`UPDATE games SET updated_at=? WHERE code='LOBY'`, time.Now().Unix()+5) // the newest

	_, out, _ := ps.adm("GET", "/api/admin/games", nil, ck)
	games := out["games"].([]any)
	if len(games) != 2 || games[0].(map[string]any)["code"] != "LOBY" || games[1].(map[string]any)["phase"] != "turn" {
		t.Fatalf("list %v", games)
	}

	// a game's page: the Keeper link is the existing ?k= link
	_, d, _ := ps.adm("GET", "/api/admin/games/PLAY", nil, ck)
	if d["keeperLink"] != "/board.html?g=PLAY&k=host" || d["canBot"] != false || d["canKick"] != false || d["canEnd"] != true ||
		d["typeCode"] != true || d["goals"] == nil || d["storiesUrl"] != "/stories.html?g=PLAY&k=host" {
		t.Fatalf("detail %v", d)
	}
	if c, _, _ := ps.adm("GET", "/api/admin/games/NOPE", nil, ck); c != 404 {
		t.Fatal("unknown game")
	}

	// bot seats: next free name and colour, lobby only
	lobby.Players[1].Name = "Bot 1"
	for _, want := range []string{"Bot 2", "Bot 3"} {
		c, out, _ := ps.adm("POST", "/api/admin/games/LOBY/bot", nil, ck)
		if c != 200 || out["name"] != want {
			t.Fatalf("bot: %d %v", c, out)
		}
	}
	got := lobby.Players[len(lobby.Players)-1]
	if got.Name != "Bot 3" || got.Color != Colors[4] || lobby.Players[3].Color != Colors[3] {
		t.Fatalf("bot seats %+v %+v", lobby.Players[3], got)
	}
	if c, _, _ := ps.adm("POST", "/api/admin/games/PLAY/bot", nil, ck); c != 400 {
		t.Fatal("a bot seat after the start")
	}
	for len(lobby.Players) < MaxPlayers {
		ps.adm("POST", "/api/admin/games/LOBY/bot", nil, ck)
	}
	if c, out, _ := ps.adm("POST", "/api/admin/games/LOBY/bot", nil, ck); c != 400 || !strings.Contains(out["error"].(string), "full") {
		t.Fatalf("a 13th seat: %d %v", c, out)
	}
	if _, d, _ := ps.adm("GET", "/api/admin/games/LOBY", nil, ck); d["canBot"] != false || d["canKick"] != true || d["typeCode"] != false {
		t.Fatalf("full lobby %v", d)
	}

	// remove a player in the lobby, with their photo; not after the start
	gone := lobby.Players[0]
	ps.s.db.Exec(`INSERT INTO photos(code, pid, data) VALUES('LOBY', ?, x'ffd8ff')`, gone.ID)
	if c, out, _ := ps.adm("POST", "/api/admin/games/LOBY/kick", map[string]string{"pid": gone.ID}, ck); c != 200 {
		t.Fatalf("kick: %d %v", c, out)
	}
	if lobby.player(gone.ID) != nil || ps.rows("LOBY", gone.ID) != 0 {
		t.Fatal("still there, or the photo stayed")
	}
	if c, _, _ := ps.adm("POST", "/api/admin/games/PLAY/kick", map[string]string{"pid": play.ps[0].ID}, ck); c != 400 {
		t.Fatal("kicked during play")
	}

	// end early: not while someone shares, then on to the guesses
	play.g.share("ring", play.ps[0], "a card", "", 3)
	if c, _, _ := ps.adm("POST", "/api/admin/games/PLAY/end", nil, ck); c != 400 {
		t.Fatal("ended during a share")
	}
	if _, d, _ := ps.adm("GET", "/api/admin/games/PLAY", nil, ck); d["canEnd"] != false || d["endWhy"] == "" {
		t.Fatalf("end while sharing %v", d)
	}
	play.g.Shares = nil
	if c, _, _ := ps.adm("POST", "/api/admin/games/PLAY/end", nil, ck); c != 200 || play.g.Phase != PhaseGuess || play.g.Result != "ended" {
		t.Fatalf("end: phase %s result %s", play.g.Phase, play.g.Result)
	}
	var n int
	ps.s.db.QueryRow(`SELECT COUNT(*) FROM events WHERE code='PLAY' AND actor='admin' AND type='endGame'`).Scan(&n)
	if n != 1 {
		t.Fatal("the end isn't logged")
	}
}

func TestAdminDeleteKeepsStories(t *testing.T) {
	f := withTelegram(t)
	ps := newPhotoServer(t)
	ck := ps.login(f)
	x := newTable(t, 6, nil)
	x.play()
	g := ps.add(x, "DELE")
	if c, out := ps.clip("DELE", "host", "clip-0001", 1, x.ps[0], nil, "audio/webm", webm(64)); c != 200 {
		t.Fatalf("clip: %d %v", c, out)
	}
	ps.s.db.Exec(`INSERT INTO photos(code, pid, data) VALUES('DELE', ?, x'ffd8ff')`, x.ps[1].ID)
	ps.add(newTable(t, 2, nil), "EMPT") // a lobby with no stories

	// still being played: the code must be typed
	if c, out, _ := ps.adm("DELETE", "/api/admin/games/DELE", map[string]string{"confirm": ""}, ck); c != 400 || !strings.Contains(out["error"].(string), "type its code") {
		t.Fatalf("delete without the code: %d %v", c, out)
	}
	if c, out, _ := ps.adm("DELETE", "/api/admin/games/DELE", map[string]string{"confirm": "dele"}, ck); c != 200 || out["storiesKept"] != 1.0 {
		t.Fatalf("delete: %d %v", c, out)
	}
	var rows int
	ps.s.db.QueryRow(`SELECT (SELECT COUNT(*) FROM games WHERE code='DELE') + (SELECT COUNT(*) FROM photos WHERE code='DELE')`).Scan(&rows)
	if ps.s.games["DELE"] != nil || rows != 0 {
		t.Fatal("the game or its photos stayed")
	}
	// a lobby goes without typing, and leaves nothing behind
	if c, _, _ := ps.adm("DELETE", "/api/admin/games/EMPT", nil, ck); c != 200 {
		t.Fatal("delete a lobby")
	}

	// the list keeps it as stories only
	_, out, _ := ps.adm("GET", "/api/admin/games", nil, ck)
	games := out["games"].([]any)
	if len(games) != 1 {
		t.Fatalf("list %v", games)
	}
	row := games[0].(map[string]any)
	if row["code"] != "DELE" || row["phase"] != "deleted" || row["stories"] != 1.0 || len(row["players"].([]any)) != 6 || row["live"] != false {
		t.Fatalf("stories-only row %v", row)
	}
	_, d, _ := ps.adm("GET", "/api/admin/games/DELE", nil, ck)
	if d["storiesUrl"] != "/stories.html?g=DELE&k=host" || d["keeperLink"] != nil {
		t.Fatalf("deleted detail %v", d)
	}
	// its stories still open: for the admin session, and for the Keeper
	if c, out, _ := ps.adm("GET", "/api/games/DELE/recordings", nil, ck); c != 200 || len(out["recordings"].([]any)) != 1 {
		t.Fatalf("admin stories: %d %v", c, out)
	}
	if len(ps.list("DELE", g.HostSecret)) != 1 {
		t.Fatal("the Keeper lost the stories")
	}
	if c, _, _ := ps.req("GET", "/api/games/DELE/recordings", map[string]string{"X-Keeper": "wrong"}, nil); c != 403 {
		t.Fatal("anyone can read the stories")
	}
	// a new game never takes a kept code
	if !ps.s.codeTaken("DELE") || ps.s.codeTaken("ZZZZ") {
		t.Fatal("codeTaken")
	}

	// deleting the stories too needs the code typed; then nothing is left
	if c, _, _ := ps.adm("DELETE", "/api/admin/games/DELE/stories", map[string]string{"confirm": "x"}, ck); c != 400 {
		t.Fatal("stories deleted without the code")
	}
	if c, out, _ := ps.adm("DELETE", "/api/admin/games/DELE/stories", map[string]string{"confirm": "DELE"}, ck); c != 200 || out["deleted"] != 1.0 {
		t.Fatalf("delete stories: %d %v", c, out)
	}
	if _, out, _ := ps.adm("GET", "/api/admin/games", nil, ck); len(out["games"].([]any)) != 0 {
		t.Fatal("still listed")
	}
	if c, _, _ := ps.adm("GET", "/api/admin/games/DELE", nil, ck); c != 404 {
		t.Fatal("still has a page")
	}
}

func TestCreatedAt(t *testing.T) {
	before := time.Now().Unix()
	g := NewGame("NEW1", "h")
	if g.CreatedAt < before || g.Rules != "world-tree-1" {
		t.Fatalf("createdAt %d rules %s", g.CreatedAt, g.Rules)
	}
	// a game saved before createdAt existed still loads, with 0
	ps := newPhotoServer(t)
	old := NewGame("OLD1", "h")
	b, _ := json.Marshal(old)
	var m map[string]any
	json.Unmarshal(b, &m)
	delete(m, "createdAt")
	b, _ = json.Marshal(m)
	ps.s.db.Exec(`INSERT INTO games(code, state, updated_at) VALUES('OLD1', ?, 1)`, string(b))
	if err := ps.s.load(); err != nil || ps.s.games["OLD1"] == nil || ps.s.games["OLD1"].CreatedAt != 0 {
		t.Fatalf("load: %v %+v", err, ps.s.games["OLD1"])
	}
}

func TestAdminPageServed(t *testing.T) {
	ps := newPhotoServer(t)
	_, _, rec := ps.adm("GET", "/admin", nil, nil)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "<title>Heartwood") || rec.Header().Get("Cache-Control") != "no-store" ||
		rec.Header().Get("X-Frame-Options") != "DENY" {
		t.Fatalf("admin page: %d %v", rec.Code, rec.Header())
	}
}
