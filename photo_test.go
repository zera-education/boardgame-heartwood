package main

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// photoServer is a real Server (temporary SQLite, the real routes) with test
// games loaded into it.
type photoServer struct {
	t *testing.T
	s *Server
	h http.Handler
}

func newPhotoServer(t *testing.T) *photoServer {
	t.Helper()
	db, err := openDB(filepath.Join(t.TempDir(), "hw.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	s := &Server{db: db, games: map[string]*Game{}, subs: map[string]map[chan struct{}]bool{}}
	return &photoServer{t, s, s.routes()}
}

// add puts a test table's game into the server under code.
func (ps *photoServer) add(x *table, code string) *Game {
	ps.t.Helper()
	x.g.Code = code
	ps.s.games[code] = x.g
	if err := ps.s.save(x.g); err != nil {
		ps.t.Fatal(err)
	}
	return x.g
}

func (ps *photoServer) do(method, path string, body any) (int, map[string]any, *httptest.ResponseRecorder) {
	ps.t.Helper()
	var rd *bytes.Reader
	if s, ok := body.(string); ok {
		rd = bytes.NewReader([]byte(s))
	} else {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	rec := httptest.NewRecorder()
	ps.h.ServeHTTP(rec, httptest.NewRequest(method, path, rd))
	var out map[string]any
	if strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json") {
		json.Unmarshal(rec.Body.Bytes(), &out)
	}
	return rec.Code, out, rec
}

func (ps *photoServer) upload(code string, p *Player, data string) (int, map[string]any) {
	ps.t.Helper()
	c, out, _ := ps.do("POST", "/api/games/"+code+"/photo", map[string]string{"pid": p.ID, "secret": p.Secret, "data": data})
	return c, out
}

func (ps *photoServer) rows(code, pid string) int {
	ps.t.Helper()
	var n int
	if err := ps.s.db.QueryRow(`SELECT COUNT(*) FROM photos WHERE code=? AND pid=?`, code, pid).Scan(&n); err != nil {
		ps.t.Fatal(err)
	}
	return n
}

func picture(w, h int) *image.RGBA {
	m := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			m.Set(x, y, color.RGBA{uint8(x), uint8(y), 120, 255})
		}
	}
	return m
}

func jpegBytes(t *testing.T, w, h int) []byte {
	var b bytes.Buffer
	if err := jpeg.Encode(&b, picture(w, h), &jpeg.Options{Quality: 82}); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func pngBytes(t *testing.T, w, h int) []byte {
	var b bytes.Buffer
	if err := png.Encode(&b, picture(w, h)); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func dataURL(mime string, b []byte) string {
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(b)
}

func signalled(ch chan struct{}) bool {
	select {
	case <-ch:
		return true
	default:
		return false
	}
}

func TestPhotoUpload(t *testing.T) {
	ps := newPhotoServer(t)
	x := newTable(t, 6, nil)
	g := ps.add(x, "PHOT")
	p, q := x.ps[0], x.ps[1]
	live := ps.s.subscribe("PHOT")
	pic := jpegBytes(t, 320, 320)

	// A JPEG from the phone: stored, a new photo number, version bumped and pushed.
	v0 := g.Version
	code, out := ps.upload("PHOT", p, dataURL("image/jpeg", pic))
	if code != 200 || out["ok"] != true {
		t.Fatalf("upload: %d %v", code, out)
	}
	if p.Photo == 0 || g.Version != v0+1 || out["photo"] != float64(p.Photo) || !signalled(live) {
		t.Fatalf("photo %d version %d→%d response %v", p.Photo, v0, g.Version, out["photo"])
	}
	if pl := out["view"].(map[string]any)["players"].([]any)[0].(map[string]any); pl["photo"] != float64(p.Photo) {
		t.Fatalf("the view's player photo %v", pl["photo"])
	}
	if buildView(g, "", "", "host")["players"].([]map[string]any)[0]["photo"] != p.Photo {
		t.Fatal("the Keeper's view doesn't carry the photo")
	}

	// Served with its type and a day's private caching.
	c, _, rec := ps.do("GET", "/api/games/PHOT/photo/"+p.ID+"?v=1", nil)
	if c != 200 || rec.Header().Get("Content-Type") != "image/jpeg" || rec.Header().Get("Cache-Control") != "private, max-age=86400" ||
		!bytes.Equal(rec.Body.Bytes(), pic) {
		t.Fatalf("GET photo: %d %q %q %d bytes", c, rec.Header().Get("Content-Type"), rec.Header().Get("Cache-Control"), rec.Body.Len())
	}
	if c, _, _ := ps.do("GET", "/api/games/PHOT/photo/"+q.ID, nil); c != 404 {
		t.Fatalf("a player without a photo: %d", c)
	}
	if c, _, _ := ps.do("GET", "/api/games/NONE/photo/"+p.ID, nil); c != 404 {
		t.Fatalf("no such game: %d", c)
	}

	// A new photo (PNG this time) gets a new number, never one used before.
	first := p.Photo
	if code, out := ps.upload("PHOT", p, dataURL("image/png", pngBytes(t, 64, 48))); code != 200 || p.Photo <= first {
		t.Fatalf("replace: %d %v photo %d", code, out, p.Photo)
	}
	if _, _, rec := ps.do("GET", "/api/games/PHOT/photo/"+p.ID, nil); rec.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("PNG served as %q", rec.Header().Get("Content-Type"))
	}

	// Refused, with a plain sentence, and nothing changes.
	before, version := p.Photo, g.Version
	bad := func(who *Player, data string, status int, msg string) {
		t.Helper()
		code, out := ps.upload("PHOT", who, data)
		if code != status || out["error"] != msg {
			t.Fatalf("want %d %q, got %d %v", status, msg, code, out)
		}
		if p.Photo != before || g.Version != version {
			t.Fatal("a refused upload changed something")
		}
	}
	notPhoto := "that isn't a photo: send a JPEG or PNG"
	bad(p, dataURL("image/jpeg", []byte("hello, this is not a picture")), 400, notPhoto)
	bad(p, "hello", 400, notPhoto)
	bad(p, "data:image/jpeg;base64,***", 400, notPhoto)
	bad(p, dataURL("image/jpeg", pic[:20]), 400, notPhoto)
	big := make([]byte, 300*1024+1)
	rand.Read(big)
	bad(p, dataURL("image/jpeg", big), 400, "that photo is too big: 300 KB at most")
	huge := make([]byte, 400*1024)
	rand.Read(huge)
	bad(p, dataURL("image/jpeg", huge), 400, "that photo is too big: 300 KB at most")
	bad(p, dataURL("image/png", pngBytes(t, 1025, 4)), 400, "that photo is too big: 1024 pixels a side at most")
	wrong := *p
	wrong.Secret = "wrong"
	bad(&wrong, dataURL("image/jpeg", pic), 403, "not recognised: rejoin the game")
	nobody := &Player{ID: "nobody", Secret: p.Secret}
	bad(nobody, dataURL("image/jpeg", pic), 403, "not recognised: rejoin the game")
	if c, out, _ := ps.do("POST", "/api/games/PHOT/photo", "{not json"); c != 400 || out["error"] != "bad request" {
		t.Fatalf("bad JSON: %d %v", c, out)
	}

	// Removing: data "" deletes it; removing nothing is fine and changes nothing.
	signalled(live)
	if code, out := ps.upload("PHOT", p, ""); code != 200 || out["photo"] != 0.0 || p.Photo != 0 || g.Version != version+1 || !signalled(live) {
		t.Fatalf("remove: %d %v photo %d", code, out, p.Photo)
	}
	if ps.rows("PHOT", p.ID) != 0 {
		t.Fatal("the removed photo is still stored")
	}
	if c, _, _ := ps.do("GET", "/api/games/PHOT/photo/"+p.ID, nil); c != 404 {
		t.Fatalf("GET a removed photo: %d", c)
	}
	if code, _ := ps.upload("PHOT", p, ""); code != 200 || g.Version != version+1 {
		t.Fatal("removing no photo bumped the version")
	}

	// Once the game has ended, photos can't change, but can still be removed.
	if code, _ := ps.upload("PHOT", q, dataURL("image/jpeg", pic)); code != 200 {
		t.Fatal("upload for q")
	}
	g.end("won")
	if code, out := ps.upload("PHOT", p, dataURL("image/jpeg", pic)); code != 400 || out["error"] != "the game has ended: photos can't be changed now" {
		t.Fatalf("upload after the end: %d %v", code, out)
	}
	if code, _ := ps.upload("PHOT", q, ""); code != 200 || q.Photo != 0 || ps.rows("PHOT", q.ID) != 0 {
		t.Fatal("removing after the end")
	}
}

// The Keeper's removePhoto goes through the action route; kicking deletes too.
func TestPhotoKeeperAndKick(t *testing.T) {
	ps := newPhotoServer(t)
	x := newTable(t, 7, nil)
	g := ps.add(x, "KEEP")
	p, q := x.ps[0], x.ps[6]
	act := func(body map[string]any) (int, map[string]any) {
		c, out, _ := ps.do("POST", "/api/games/KEEP/action", body)
		return c, out
	}
	pic := dataURL("image/jpeg", jpegBytes(t, 320, 320))
	for _, who := range []*Player{p, q} {
		if code, out := ps.upload("KEEP", who, pic); code != 200 {
			t.Fatalf("upload: %d %v", code, out)
		}
	}

	if c, out := act(map[string]any{"pid": p.ID, "secret": p.Secret, "type": "removePhoto", "target": p.ID}); c != 400 || out["error"] != "unknown action" {
		t.Fatalf("a phone using removePhoto: %d %v", c, out)
	}
	if c, out := act(map[string]any{"host": "host", "type": "removePhoto", "target": "nobody"}); c != 400 || out["error"] != "no such player" {
		t.Fatalf("removePhoto for nobody: %d %v", c, out)
	}
	version := g.Version
	live := ps.s.subscribe("KEEP")
	if c, out := act(map[string]any{"host": "host", "type": "removePhoto", "target": p.ID}); c != 200 || out["ok"] != true {
		t.Fatalf("removePhoto: %d %v", c, out)
	}
	if p.Photo != 0 || ps.rows("KEEP", p.ID) != 0 || g.Version != version+1 || !signalled(live) {
		t.Fatalf("after removePhoto: photo %d rows %d version %d", p.Photo, ps.rows("KEEP", p.ID), g.Version)
	}
	if c, _, _ := ps.do("GET", "/api/games/KEEP/photo/"+p.ID, nil); c != 404 {
		t.Fatal("the removed photo is still served")
	}
	if c, out := act(map[string]any{"host": "host", "type": "removePhoto", "target": p.ID}); c != 400 || out["error"] != "P0 has no photo" {
		t.Fatalf("removing twice: %d %v", c, out)
	}

	// Kicking a player in the lobby deletes their photo at once.
	if ps.rows("KEEP", q.ID) != 1 {
		t.Fatal("q's photo isn't stored")
	}
	if c, out := act(map[string]any{"host": "host", "type": "kick", "target": q.ID}); c != 200 {
		t.Fatalf("kick: %d %v", c, out)
	}
	if ps.rows("KEEP", q.ID) != 0 || g.player(q.ID) != nil {
		t.Fatal("the kicked player's photo is still stored")
	}
}

// The sweep deletes photos 2 hours after the result, after 24 hours without a
// change, and of players or games that are gone.
func TestPhotoSweep(t *testing.T) {
	ps := newPhotoServer(t)
	now := time.Now()
	pic := dataURL("image/jpeg", jpegBytes(t, 32, 32))
	games := map[string]*table{}
	for _, code := range []string{"ENDD", "JUST", "IDLE", "PLAY"} {
		x := newTable(t, 6, nil)
		ps.add(x, code)
		games[code] = x
		for _, p := range x.ps[:2] {
			if c, out := ps.upload(code, p, pic); c != 200 {
				t.Fatalf("upload %s: %d %v", code, c, out)
			}
		}
	}
	games["ENDD"].g.end("won")
	games["ENDD"].g.EndedAt = now.Add(-2*time.Hour - time.Minute).Unix()
	games["JUST"].g.end("lost")
	games["JUST"].g.EndedAt = now.Add(-time.Hour).Unix()
	if _, err := ps.s.db.Exec(`UPDATE games SET updated_at=? WHERE code='IDLE'`, now.Add(-25*time.Hour).Unix()); err != nil {
		t.Fatal(err)
	}
	// Leftovers: a player no longer in the game, and a game no longer loaded.
	play := games["PLAY"]
	ps.s.db.Exec(`INSERT INTO photos(code,pid,data) VALUES('PLAY','gone1','x'), ('LOST','p1','x')`)

	ended := games["ENDD"].g
	version := ended.Version
	live := ps.s.subscribe("ENDD")
	ps.s.sweepPhotos(now)

	for _, code := range []string{"ENDD", "IDLE"} {
		x := games[code]
		for _, p := range x.ps[:2] {
			if ps.rows(code, p.ID) != 0 || p.Photo != 0 {
				t.Fatalf("%s: %s's photo survived the sweep (photo %d)", code, p.Name, p.Photo)
			}
		}
		if v := buildView(x.g, "", "", "host")["players"].([]map[string]any)[0]["photo"]; v != 0 {
			t.Fatalf("%s: view photo %v", code, v)
		}
	}
	if ended.Version != version+1 || !signalled(live) {
		t.Fatal("the sweep didn't push the change")
	}
	for _, code := range []string{"JUST", "PLAY"} {
		for _, p := range games[code].ps[:2] {
			if ps.rows(code, p.ID) != 1 || p.Photo == 0 {
				t.Fatalf("%s: %s's photo was swept too early", code, p.Name)
			}
		}
	}
	if ps.rows("PLAY", "gone1") != 0 || ps.rows("LOST", "p1") != 0 {
		t.Fatal("leftover photos survived the sweep")
	}

	// Photo numbers and the end time are saved with the game; old saves load as zero.
	s2 := &Server{db: ps.s.db, games: map[string]*Game{}, subs: map[string]map[chan struct{}]bool{}}
	if err := s2.load(); err != nil {
		t.Fatal(err)
	}
	if s2.games["PLAY"].Players[0].Photo != play.ps[0].Photo || s2.games["ENDD"].EndedAt != ended.EndedAt || s2.games["PLAY"].EndedAt != 0 {
		t.Fatal("photo or ended at not saved")
	}
	var old Game
	if err := json.Unmarshal([]byte(`{"rules":"world-tree-1","result":"won","players":[{"id":"a","name":"A"}]}`), &old); err != nil ||
		old.Rules != RulesVersion || old.EndedAt != 0 || old.Players[0].Photo != 0 {
		t.Fatal("an old save doesn't load")
	}
}
