package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// fakeJPEG is a stand-in picture: the JPEG magic, then anything.
var fakeJPEG = append([]byte{0xff, 0xd8, 0xff, 0xe0}, bytes.Repeat([]byte{7}, 40)...)

func TestSealedFind(t *testing.T) {
	t.Cleanup(func() { sealed.Store(nil) })
	x := newTable(t, 6, nil)
	x.play()
	g, mira, mate := x.g, x.ps[0], x.ps[1]
	mira.Name = "mira Tan"
	// explore puts p on a fresh face-down hex of this kind and explores it
	var fresh []int
	for s := range 6 {
		fresh = append(fresh, ringHexes(3, s)...)
	}
	explore := func(p *Player, kind string) {
		t.Helper()
		x.turnOf(p)
		p.Pos, fresh = fresh[0], fresh[1:]
		g.Tiles[p.Pos] = &Tile{Kind: kind}
		x.must(x.as(p, "explore", own))
	}

	// no sealed find on this server: explore is as it always was
	x.turnOf(mira)
	x.must(x.as(mira, "explore", own))
	if g.Found != nil || !strings.HasSuffix(g.Log[len(g.Log)-1], "finds nothing here.") {
		t.Fatalf("found %v, log %q", g.Found, g.Log[len(g.Log)-1])
	}

	sealed.Store(&SealedFind{Name: "Mira", Cheer: "Hooray", Image: fakeJPEG})
	// someone else's explores don't count; Mira's first two find what is there
	for range FoundAfter {
		explore(mate, "empty")
	}
	explore(mira, "spring")
	explore(mira, "empty")
	if g.Found != nil || g.FoundTries != 2 || !strings.HasSuffix(g.Log[len(g.Log)-1], "mira Tan explores and finds nothing here.") {
		t.Fatalf("found %v after %d explores, log %q", g.Found, g.FoundTries, g.Log[len(g.Log)-1])
	}
	// her third is a spring: it waits for her next empty hex
	explore(mira, "spring")
	if g.Found != nil || !strings.HasSuffix(g.Log[len(g.Log)-1], "finds a spring 💧.") {
		t.Fatal("found on a spring")
	}
	// Mira on an empty hex turns it up, and every screen shows it
	explore(mira, "empty")
	if g.Found == nil || g.Found.Player != mira.ID || g.Found.Hex != mira.Pos || g.Found.Stage != "ask" {
		t.Fatalf("found %+v", g.Found)
	}
	if l := g.Log[len(g.Log)-1]; l != "mira Tan explores and finds something… unexpected!?" {
		t.Fatalf("log %q", l)
	}
	for _, v := range []map[string]any{buildView(g, "", "", "host"), buildView(g, mate.ID, mate.Secret, ""), buildView(g, "", "", "")} {
		f, _ := v["found"].(map[string]any)
		if f["player"] != mira.ID || f["stage"] != "ask" || f["question"] != FoundQuestion || f["cheer"] != "Hooray" ||
			!strings.HasPrefix(f["image"].(string), "/api/games/TEST/found?v=") {
			t.Fatalf("view %v", f)
		}
	}
	// once a game
	explore(mira, "empty")
	if g.Found.Hex == mira.Pos {
		t.Fatal("found twice")
	}
	// Undo takes it back with the explore
	x.must(x.host("undo", Action{}))
	saved := *g.Found
	x.turnOf(mira)
	x.must(x.as(mira, "explore", own))
	x.must(x.host("undo", Action{}))
	if *g.Found != saved {
		t.Fatal("undo changed the find")
	}
	g.Found = nil
	x.must(x.as(mira, "explore", own))
	x.must(x.host("undo", Action{}))
	if g.Found != nil {
		t.Fatal("undo kept the find")
	}
	x.must(x.as(mira, "explore", own))
	for _, r := range g.Recognition() {
		if (r.Surprise != "") != (r.ID == mira.ID) {
			t.Fatalf("%s surprise %q", r.Name, r.Surprise)
		}
	}
	// the Keeper moves it on: the celebration, then closed
	x.fails(x.phone(mira, "foundCheer", own), "a phone moving the find on")
	x.must(x.host("foundCheer", Action{}))
	if f := buildView(g, "", "", "")["found"].(map[string]any); f["stage"] != "cheer" {
		t.Fatalf("stage %v", f["stage"])
	}
	x.must(x.host("foundClose", Action{}))
	if _, ok := buildView(g, "", "", "")["found"]; ok {
		t.Fatal("closed but still showing")
	}
	x.fails(x.host("foundCheer", Action{}), "moving a closed find on")
	// a server without the find shows nothing, even for a game that turned it up
	g.Found.Stage = "ask"
	sealed.Store(nil)
	if _, ok := buildView(g, "", "", "")["found"]; ok {
		t.Fatal("showing a find the server no longer has")
	}
	// the name it waits for
	f := &SealedFind{Name: "Mira", Image: fakeJPEG}
	for name, want := range map[string]bool{"Mira": true, "  MIRA ": true, "mira tan": true, "Mira!": true, "Miranda": false, "Tan Mira": false, "": false} {
		if f.calls(name) != want {
			t.Errorf("calls(%q) = %v", name, !want)
		}
	}
	if (*SealedFind)(nil).calls("Mira") || (&SealedFind{}).calls("") {
		t.Error("calls with no find")
	}
}

func TestSealedFindAPI(t *testing.T) {
	t.Cleanup(func() { sealed.Store(nil) })
	ps := newPhotoServer(t)
	ps.s.findPath = filepath.Join(t.TempDir(), "sealed-find.json")
	sum := sha256.Sum256([]byte("test-token"))
	savedHash := transcribeTokenHash
	transcribeTokenHash = hex.EncodeToString(sum[:])
	t.Cleanup(func() { transcribeTokenHash = savedHash })
	auth := map[string]string{"Authorization": "Bearer test-token", "Content-Type": "application/json"}
	body := func(name string, img []byte) []byte {
		b, _ := json.Marshal(map[string]any{"name": name, "cheer": "Hooray", "image": base64.StdEncoding.EncodeToString(img)})
		return b
	}

	// only the Mini puts it there, and only a picture with a name
	if c, _, _ := ps.req("PUT", "/api/found", map[string]string{"Authorization": "Bearer nope"}, body("Mira", fakeJPEG)); c != 401 {
		t.Fatalf("put without the token: %d", c)
	}
	if c, _, _ := ps.req("PUT", "/api/found", auth, body("", fakeJPEG)); c != 400 {
		t.Fatalf("put without a name: %d", c)
	}
	if c, _, _ := ps.req("PUT", "/api/found", auth, body("Mira", []byte("not a picture"))); c != 400 {
		t.Fatalf("put without a picture: %d", c)
	}
	if sealed.Load() != nil {
		t.Fatal("a refused put took effect")
	}
	if c, out, _ := ps.req("PUT", "/api/found", auth, body("Mira", fakeJPEG)); c != 200 || out["bytes"] != float64(len(fakeJPEG)) {
		t.Fatalf("put: %d %v", c, out)
	}
	if f := sealed.Load(); f == nil || f.Name != "Mira" || !bytes.Equal(f.Image, fakeJPEG) {
		t.Fatalf("sealed %+v", f)
	}
	// it survives a restart
	sealed.Store(nil)
	if err := loadFind(ps.s.findPath); err != nil || sealed.Load() == nil || sealed.Load().Cheer != "Hooray" {
		t.Fatalf("load: %v %+v", err, sealed.Load())
	}

	// the picture only for a game that turned it up
	x := newTable(t, 6, nil)
	x.play()
	g := ps.add(x, "FIND")
	if c, _, _ := ps.req("GET", "/api/games/FIND/found", nil, nil); c != 404 {
		t.Fatalf("picture before it is found: %d", c)
	}
	x.ps[0].Name = "Mira"
	g.FoundTries = FoundAfter - 1 // her third explore
	x.turnOf(x.ps[0])
	x.must(x.as(x.ps[0], "explore", own))
	if c, _, rec := ps.req("GET", "/api/games/FIND/found", nil, nil); c != 200 || !bytes.Equal(rec.Body.Bytes(), fakeJPEG) || rec.Header().Get("Content-Type") != "image/jpeg" {
		t.Fatalf("picture: %d %q", c, rec.Header().Get("Content-Type"))
	}
	if g.Found == nil {
		t.Fatal("not found")
	}

	// taken away: gone from the server and from the screens
	if c, _, _ := ps.req("DELETE", "/api/found", nil, nil); c != 401 {
		t.Fatalf("delete without the token: %d", c)
	}
	if c, _, _ := ps.req("DELETE", "/api/found", auth, nil); c != 200 {
		t.Fatalf("delete: %d", c)
	}
	if c, _, _ := ps.req("GET", "/api/games/FIND/found", nil, nil); c != 404 || sealed.Load() != nil {
		t.Fatalf("picture after delete: %d", c)
	}
	if err := loadFind(ps.s.findPath); err != nil || sealed.Load() != nil {
		t.Fatalf("load after delete: %v", err)
	}
}
