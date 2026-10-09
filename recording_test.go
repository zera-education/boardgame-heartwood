package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// webm is a stand-in clip: the WebM (EBML) magic, then anything.
func webm(n int) []byte {
	return append([]byte{0x1a, 0x45, 0xdf, 0xa3}, bytes.Repeat([]byte{7}, n)...)
}

// req sends one request through the real routes, with headers.
func (ps *photoServer) req(method, path string, headers map[string]string, body []byte) (int, map[string]any, *httptest.ResponseRecorder) {
	ps.t.Helper()
	r := httptest.NewRequest(method, path, bytes.NewReader(body))
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	ps.h.ServeHTTP(rec, r)
	var out map[string]any
	if strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json") {
		json.Unmarshal(rec.Body.Bytes(), &out)
	}
	return rec.Code, out, rec
}

// clip uploads a recording of share idx the way web/recorder.js does.
func (ps *photoServer) clip(code, secret, clip string, idx int, p *Player, extra url.Values, ctype string, audio []byte) (int, map[string]any) {
	ps.t.Helper()
	q := url.Values{"clip": {clip}, "share": {itoa(idx)}, "kind": {"why"}, "player": {p.ID}, "value": {"2"},
		"prompt": {"Why do you stand for Resilience?"}, "sub": {"Anti-fragile."}, "round": {"0"}, "ms": {"8200"}}
	for k, v := range extra {
		q[k] = v
	}
	c, out, _ := ps.req("POST", "/api/games/"+code+"/recordings?"+q.Encode(), map[string]string{"Content-Type": ctype, "X-Keeper": secret}, audio)
	return c, out
}

func itoa(n int) string { b, _ := json.Marshal(n); return string(b) }

func (ps *photoServer) list(code, secret string) []Recording {
	ps.t.Helper()
	c, _, rec := ps.req("GET", "/api/games/"+code+"/recordings", map[string]string{"X-Keeper": secret}, nil)
	if c != 200 {
		ps.t.Fatalf("list: %d %s", c, rec.Body)
	}
	var out struct{ Recordings []Recording }
	json.Unmarshal(rec.Body.Bytes(), &out)
	return out.Recordings
}

func TestRecordingSetting(t *testing.T) {
	x := newTable(t, 6, nil)
	g := x.g
	if v := buildView(g, "", "", "host"); v["recording"] != true || v["recDevice"] != "" {
		t.Fatalf("a new game records: %v %v", v["recording"], v["recDevice"])
	}
	p := x.ps[0]
	if _, ok := buildView(g, p.ID, p.Secret, "")["recDevice"]; ok {
		t.Fatal("phones don't need to know which device records")
	}
	x.must(x.host("record", Action{N: 0}))
	if !g.NoRecord || buildView(g, p.ID, p.Secret, "")["recording"] != false || !strings.Contains(g.Log[len(g.Log)-1], "recording off") {
		t.Fatalf("switched off: %v, log %q", g.NoRecord, g.Log[len(g.Log)-1])
	}
	x.must(x.host("record", Action{N: 1}))
	if g.NoRecord || !strings.Contains(g.Log[len(g.Log)-1], "recording on") {
		t.Fatal("switched back on")
	}
	x.fails(x.host("recordHere", Action{Text: "x"}), "a bad device id")
	x.fails(x.host("recordHere", Action{Text: "bad id with spaces"}), "a bad device id")
	x.must(x.host("recordHere", Action{Text: "laptop-1234"}))
	if g.RecDevice != "laptop-1234" || buildView(g, "", "", "host")["recDevice"] != "laptop-1234" {
		t.Fatalf("device %q", g.RecDevice)
	}
	x.fails(x.phone(p, "record", Action{N: 0}), "a player switching recording")
	x.fails(g.Apply(Action{Type: "record", Host: "wrong"}), "a wrong Keeper secret")

	// Saved and loaded with the game; a game saved before recording existed records.
	b, _ := json.Marshal(g)
	var back Game
	json.Unmarshal(b, &back)
	if back.RecDevice != "laptop-1234" || back.NoRecord || back.Rules != RulesVersion {
		t.Fatalf("round trip %q %v", back.RecDevice, back.NoRecord)
	}
	old := strings.Replace(string(b), `"recDevice":"laptop-1234"`, `"x":1`, 1)
	var older Game
	if err := json.Unmarshal([]byte(old), &older); err != nil || older.NoRecord || older.RecDevice != "" {
		t.Fatalf("an older save: %v %v %q", err, older.NoRecord, older.RecDevice)
	}
}

func TestRecordingUpload(t *testing.T) {
	ps := newPhotoServer(t)
	x := newTable(t, 6, nil)
	x.play() // six "why" shares, opened and closed
	g := ps.add(x, "RECD")
	p := x.ps[2]
	audio := webm(3000)

	code, out := ps.clip("RECD", "host", "clip-aaaa-1111", 3, p, nil, "audio/webm;codecs=opus", audio)
	if code != 200 || out["ok"] != true || out["id"] == nil {
		t.Fatalf("upload: %d %v", code, out)
	}
	id := int64(out["id"].(float64))
	// The same clip again (the answer got lost, the recorder retried): stored once.
	if code, again := ps.clip("RECD", "host", "clip-aaaa-1111", 3, p, nil, "audio/webm;codecs=opus", audio); code != 200 || again["id"] != out["id"] {
		t.Fatalf("retry: %d %v", code, again)
	}
	// A treasure answer: round, seq, of; Safari sends MP4.
	mp4 := append([]byte{0, 0, 0, 0x20}, append([]byte("ftypM4A "), bytes.Repeat([]byte{1}, 500)...)...)
	tre := url.Values{"kind": {"treasure"}, "sub": {"compass"}, "seq": {"2"}, "of": {"6"}, "round": {"3"}, "value": {""}, "prompt": {"Everyone, one sentence"}}
	if code, out := ps.clip("RECD", "host", "clip-bbbb-2222", 5, x.ps[4], tre, "audio/mp4", mp4); code != 200 {
		t.Fatalf("mp4: %d %v", code, out)
	}

	rs := ps.list("RECD", "host")
	if len(rs) != 2 {
		t.Fatalf("%d recordings", len(rs))
	}
	r := rs[0]
	if r.ID != id || r.Share != 3 || r.Kind != "why" || r.Player != p.ID || r.PlayerName != p.Name || r.PlayerColor != p.Color ||
		r.Value != 2 || r.Prompt != "Why do you stand for Resilience?" || r.Sub != "Anti-fragile." || r.Mime != "audio/webm; codecs=opus" ||
		r.Bytes != len(audio) || r.DurationMs != 8200 || r.Transcript != nil || r.Status != "waiting" || !r.Audio || r.CreatedAt == 0 {
		t.Fatalf("why clip %+v", r)
	}
	if t2 := rs[1]; t2.Kind != "treasure" || t2.Sub != "compass" || t2.Seq != 2 || t2.Of != 6 || t2.Round != 3 || t2.Value != -1 || t2.Mime != "audio/mp4" {
		t.Fatalf("treasure clip %+v", t2)
	}

	// The audio, for the Keeper's player (?host= works too), with Range for seeking.
	c, _, rec := ps.req("GET", "/api/games/recd/recordings/"+itoa(int(id))+"/audio?host=host", nil, nil)
	if c != 200 || rec.Header().Get("Content-Type") != "audio/webm; codecs=opus" || !bytes.Equal(rec.Body.Bytes(), audio) ||
		!strings.Contains(rec.Header().Get("Content-Disposition"), "RECD-story-3.webm") {
		t.Fatalf("audio: %d %q %d bytes %q", c, rec.Header().Get("Content-Type"), rec.Body.Len(), rec.Header().Get("Content-Disposition"))
	}
	if c, _, rec := ps.req("GET", "/api/games/RECD/recordings/"+itoa(int(id))+"/audio?host=host", map[string]string{"Range": "bytes=0-3"}, nil); c != 206 || rec.Body.Len() != 4 {
		t.Fatalf("range: %d %d", c, rec.Body.Len())
	}

	// Refused, with a plain sentence.
	bad := func(code, secret, clip string, idx int, who *Player, ctype string, body []byte, status int, msg string) {
		t.Helper()
		c, out := ps.clip(code, secret, clip, idx, who, nil, ctype, body)
		if c != status || out["error"] != msg {
			t.Fatalf("want %d %q, got %d %v", status, msg, c, out)
		}
	}
	bad("RECD", "wrong", "clip-cccc-3333", 3, p, "audio/webm", audio, 403, "only the Keeper can do that")
	bad("RECD", "", "clip-cccc-3333", 3, p, "audio/webm", audio, 403, "only the Keeper can do that")
	bad("NONE", "host", "clip-cccc-3333", 3, p, "audio/webm", audio, 404, "no game with that code")
	bad("RECD", "host", "clip-cccc-3333", 3, p, "text/plain", audio, 415, "that isn't a recording: send WebM, MP4 or Ogg audio")
	bad("RECD", "host", "clip-cccc-3333", 3, p, "audio/mp4", audio, 415, "that isn't a recording: send WebM, MP4 or Ogg audio")
	bad("RECD", "host", "clip-cccc-3333", 3, p, "audio/webm", []byte("hello"), 415, "that isn't a recording: send WebM, MP4 or Ogg audio")
	bad("RECD", "host", "clip-cccc-3333", 3, p, "audio/webm", nil, 415, "that isn't a recording: send WebM, MP4 or Ogg audio")
	bad("RECD", "host", "short", 3, p, "audio/webm", audio, 400, "bad recording: it needs a clip id, a share and its kind")
	bad("RECD", "host", "clip-cccc-3333", 0, p, "audio/webm", audio, 400, "bad recording: it needs a clip id, a share and its kind")
	bad("RECD", "host", "clip-cccc-3333", g.ShareSeq+1, p, "audio/webm", audio, 404, "no such share in this game")
	bad("RECD", "host", "clip-cccc-3333", 3, &Player{ID: "nobody"}, "audio/webm", audio, 400, "no such player")
	bad("RECD", "host", "clip-cccc-3333", 3, p, "audio/webm", webm(maxRecordingBytes), 413, "that recording is too long: 25 MB at most")
	if c, out := ps.clip("RECD", "host", "clip-cccc-3333", 3, p, url.Values{"kind": {"gossip"}}, "audio/webm", audio); c != 400 {
		t.Fatalf("unknown kind: %d %v", c, out)
	}
	if len(ps.list("RECD", "host")) != 2 {
		t.Fatal("a refused upload was stored")
	}
	if c, _, _ := ps.req("GET", "/api/games/RECD/recordings", map[string]string{"X-Keeper": "wrong"}, nil); c != 403 {
		t.Fatalf("a stranger listing: %d", c)
	}
	if c, _, _ := ps.req("GET", "/api/games/RECD/recordings/"+itoa(int(id))+"/audio", nil, nil); c != 403 {
		t.Fatalf("a stranger listening: %d", c)
	}

	// "Don't keep this one": the Keeper deletes a clip.
	if c, _, _ := ps.req("DELETE", "/api/games/RECD/recordings/"+itoa(int(id)), map[string]string{"X-Keeper": "wrong"}, nil); c != 403 {
		t.Fatalf("a stranger deleting: %d", c)
	}
	if c, out, _ := ps.req("DELETE", "/api/games/RECD/recordings/"+itoa(int(id)), map[string]string{"X-Keeper": "host"}, nil); c != 200 || out["ok"] != true {
		t.Fatalf("delete: %d %v", c, out)
	}
	if rs := ps.list("RECD", "host"); len(rs) != 1 || rs[0].Kind != "treasure" {
		t.Fatalf("after delete: %+v", rs)
	}
	if c, _, _ := ps.req("DELETE", "/api/games/RECD/recordings/"+itoa(int(id)), map[string]string{"X-Keeper": "host"}, nil); c != 404 {
		t.Fatalf("delete twice: %d", c)
	}
	if c, _, _ := ps.req("GET", "/api/games/RECD/recordings/"+itoa(int(id))+"/audio?host=host", nil, nil); c != 404 {
		t.Fatalf("a deleted clip's audio: %d", c)
	}

	// Another game's Keeper can't see or delete this game's stories.
	y := newTable(t, 6, nil)
	other := ps.add(y, "OTHR")
	other.HostSecret = "other"
	if rs := ps.list("OTHR", "other"); len(rs) != 0 {
		t.Fatalf("another game's list: %+v", rs)
	}
	treasure := ps.list("RECD", "host")[0].ID
	if c, _, _ := ps.req("DELETE", "/api/games/OTHR/recordings/"+itoa(int(treasure)), map[string]string{"X-Keeper": "other"}, nil); c != 404 {
		t.Fatalf("deleting through another game: %d", c)
	}

	// A game that is no longer loaded (saved under older rules) keeps its stories for its Keeper.
	delete(ps.s.games, "RECD")
	if rs := ps.list("RECD", "host"); len(rs) != 1 {
		t.Fatalf("an unloaded game's stories: %d", len(rs))
	}
	if c, _, _ := ps.req("GET", "/api/games/RECD/recordings", map[string]string{"X-Keeper": "wrong"}, nil); c != 403 {
		t.Fatalf("an unloaded game, wrong secret: %d", c)
	}
	if c, out := ps.clip("RECD", "host", "clip-dddd-4444", 3, p, nil, "audio/webm", audio); c != 404 {
		t.Fatalf("uploading to an unloaded game: %d %v", c, out)
	}
}

func TestTranscribeWorker(t *testing.T) {
	ps := newPhotoServer(t)
	x := newTable(t, 6, nil)
	x.play()
	ps.add(x, "TRAN")
	sum := sha256.Sum256([]byte("test-token"))
	saved := transcribeTokenHash
	transcribeTokenHash = hex.EncodeToString(sum[:])
	t.Cleanup(func() { transcribeTokenHash = saved })
	auth := map[string]string{"Authorization": "Bearer test-token"}
	post := func(id int64, body string) (int, map[string]any) {
		t.Helper()
		c, out, _ := ps.req("POST", "/api/transcribe/"+itoa(int(id)), map[string]string{"Authorization": "Bearer test-token", "Content-Type": "application/json"}, []byte(body))
		return c, out
	}
	queue := func() []map[string]any {
		t.Helper()
		c, out, rec := ps.req("GET", "/api/transcribe/queue", auth, nil)
		if c != 200 {
			t.Fatalf("queue: %d %s", c, rec.Body)
		}
		var items []map[string]any
		for _, it := range out["recordings"].([]any) {
			items = append(items, it.(map[string]any))
		}
		return items
	}

	if len(queue()) != 0 {
		t.Fatal("nothing recorded yet")
	}
	for i, clip := range []string{"clip-one-11111", "clip-two-22222", "clip-three-333"} {
		if c, out := ps.clip("TRAN", "host", clip, i+1, x.ps[i], nil, "audio/webm", webm(100+i)); c != 200 {
			t.Fatalf("upload %d: %d %v", i, c, out)
		}
	}
	rs := ps.list("TRAN", "host")
	a, b, c3 := rs[0].ID, rs[1].ID, rs[2].ID

	// Only the worker, with its token.
	for _, h := range []map[string]string{nil, {"Authorization": "Bearer wrong"}, {"Authorization": "test-token"}, {"X-Keeper": "host"}} {
		if c, _, _ := ps.req("GET", "/api/transcribe/queue", h, nil); c != 401 {
			t.Fatalf("queue with %v: %d", h, c)
		}
	}
	if c, _, _ := ps.req("GET", "/api/transcribe/"+itoa(int(a))+"/audio", nil, nil); c != 401 {
		t.Fatalf("audio without the token: %d", c)
	}
	if c, _, _ := ps.req("POST", "/api/transcribe/"+itoa(int(a)), map[string]string{"Authorization": "Bearer nope"}, []byte(`{"text":"x"}`)); c != 401 {
		t.Fatalf("posting without the token: %d", c)
	}

	q := queue()
	if len(q) != 3 || q[0]["id"] != float64(a) || q[0]["code"] != "TRAN" || q[0]["mime"] != "audio/webm" || q[0]["bytes"] != float64(104) {
		t.Fatalf("queue %v", q)
	}
	if q[1]["share"] != float64(2) || q[1]["kind"] != rs[1].Kind || q[1]["playerName"] != x.ps[1].Name || q[1]["prompt"] != rs[1].Prompt {
		t.Fatalf("what the story was: %v", q[1])
	}
	cd, _, rec := ps.req("GET", "/api/transcribe/"+itoa(int(b))+"/audio", auth, nil)
	if cd != 200 || !bytes.Equal(rec.Body.Bytes(), webm(101)) || rec.Header().Get("Content-Type") != "audio/webm" {
		t.Fatalf("worker audio: %d %d bytes", cd, rec.Body.Len())
	}

	// A transcript: stored with its language, and Whisper's duration.
	if c, out := post(a, `{"text":"  I stand for Resilience because my grandmother never gave up. ","language":"en","duration":5.7}`); c != 200 || out["ok"] != true {
		t.Fatalf("post text: %d %v", c, out)
	}
	// Nothing said: an empty transcript is a result too.
	if c, out := post(b, `{"text":"","language":"en","duration":2}`); c != 200 {
		t.Fatalf("post empty: %d %v", c, out)
	}
	// Errors count as tries; after three the clip is left alone.
	for k := 1; k <= 3; k++ {
		if c, out := post(c3, `{"error":"could not decode the audio"}`); c != 200 || out["attempts"] != float64(k) {
			t.Fatalf("error %d: %d %v", k, c, out)
		}
	}
	if q := queue(); len(q) != 0 {
		t.Fatalf("queue after: %v", q)
	}
	rs = ps.list("TRAN", "host")
	if r := rs[0]; r.Status != "done" || r.Transcript == nil || *r.Transcript != "I stand for Resilience because my grandmother never gave up." ||
		r.Language != "en" || r.DurationMs != 5700 || r.TranscribedAt == 0 {
		t.Fatalf("transcribed %+v", r)
	}
	if r := rs[1]; r.Status != "done" || r.Transcript == nil || *r.Transcript != "" || r.DurationMs != 2000 {
		t.Fatalf("silent %+v", r)
	}
	if r := rs[2]; r.Status != "failed" || r.Attempts != 3 || r.Error != "could not decode the audio" || r.Transcript != nil {
		t.Fatalf("failed %+v", r)
	}
	// The Keeper asks for another try.
	if c, _, _ := ps.req("POST", "/api/games/TRAN/recordings/"+itoa(int(c3))+"/retry", map[string]string{"X-Keeper": "wrong"}, nil); c != 403 {
		t.Fatalf("retry by a stranger: %d", c)
	}
	if c, out, _ := ps.req("POST", "/api/games/TRAN/recordings/"+itoa(int(c3))+"/retry", map[string]string{"X-Keeper": "host"}, nil); c != 200 || out["ok"] != true {
		t.Fatalf("retry: %d %v", c, out)
	}
	if q := queue(); len(q) != 1 || q[0]["id"] != float64(c3) || q[0]["attempts"] != float64(0) {
		t.Fatalf("queue after retry: %v", q)
	}
	// Bad results.
	if c, _ := post(c3, `{}`); c != 400 {
		t.Fatalf("empty result: %d", c)
	}
	if c, _ := post(c3, `not json`); c != 400 {
		t.Fatalf("bad json: %d", c)
	}
	if c, _ := post(99999, `{"text":"hello"}`); c != 404 {
		t.Fatalf("no such recording: %d", c)
	}
	if c, _ := post(a, `{"error":"late error"}`); c != 404 {
		t.Fatalf("an error for a clip that has its transcript: %d", c)
	}
	// The Keeper asks again for a transcribed clip whose text came out wrong.
	if c, _, _ := ps.req("POST", "/api/games/TRAN/recordings/"+itoa(int(a))+"/retry", map[string]string{"X-Keeper": "host"}, nil); c != 200 {
		t.Fatalf("transcribing a clip again: %d", c)
	}
	if q := queue(); len(q) != 2 || q[0]["id"] != float64(a) {
		t.Fatalf("queue after transcribing again: %v", q)
	}
	if r := ps.list("TRAN", "host")[0]; r.Status != "waiting" || r.Transcript != nil || r.Language != "" || r.TranscribedAt != 0 {
		t.Fatalf("waiting again %+v", r)
	}
	if c, _, _ := ps.req("POST", "/api/games/TRAN/recordings/"+itoa(int(c3))+"/retry", map[string]string{"X-Keeper": "host"}, nil); c != 200 {
		t.Fatalf("asking twice: %d", c)
	}

	// The worker API is off when the server has no token hash.
	transcribeTokenHash = ""
	if c, _, _ := ps.req("GET", "/api/transcribe/queue", auth, nil); c != 503 {
		t.Fatalf("no token configured: %d", c)
	}
}

func TestRecordingSweep(t *testing.T) {
	ps := newPhotoServer(t)
	x := newTable(t, 6, nil)
	x.play()
	g := ps.add(x, "SWEP")
	for i, clip := range []string{"clip-old-done-1", "clip-new-done-2", "clip-old-wait-3"} {
		if c, out := ps.clip("SWEP", "host", clip, i+1, x.ps[i], nil, "audio/webm", webm(50)); c != 200 {
			t.Fatalf("upload %d: %d %v", i, c, out)
		}
	}
	now := time.Now()
	day := int64(24 * 3600)
	db := ps.s.db
	db.Exec(`UPDATE recordings SET transcript='old story', transcribed_at=?, created_at=? WHERE clip='clip-old-done-1'`, now.Unix()-31*day, now.Unix()-31*day)
	db.Exec(`UPDATE recordings SET transcript='new story', transcribed_at=? WHERE clip='clip-new-done-2'`, now.Unix()-29*day)
	db.Exec(`UPDATE recordings SET created_at=? WHERE clip='clip-old-wait-3'`, now.Unix()-60*day)

	// The photo sweep (game over long ago) leaves recordings alone: they are the team's story record.
	g.end("won")
	g.EndedAt = now.Unix() - 40*day
	ps.s.sweepPhotos(now)
	if n := len(ps.list("SWEP", "host")); n != 3 {
		t.Fatalf("the photo sweep removed recordings: %d left", n)
	}

	ps.s.sweepRecordings(now)
	rs := ps.list("SWEP", "host")
	if len(rs) != 3 {
		t.Fatalf("%d recordings after the sweep", len(rs))
	}
	if r := rs[0]; r.Audio || r.Transcript == nil || *r.Transcript != "old story" || r.Status != "done" {
		t.Fatalf("transcribed 31 days ago: %+v", r)
	}
	var deletedAt int64
	db.QueryRow(`SELECT COALESCE(audio_deleted_at,0) FROM recordings WHERE clip='clip-old-done-1'`).Scan(&deletedAt)
	if deletedAt != now.Unix() {
		t.Fatalf("audio_deleted_at %d", deletedAt)
	}
	if c, out, _ := ps.req("GET", "/api/games/SWEP/recordings/"+itoa(int(rs[0].ID))+"/audio?host=host", nil, nil); c != 404 || !strings.Contains(out["error"].(string), "transcript is kept") {
		t.Fatalf("deleted audio: %d %v", c, out)
	}
	if !rs[1].Audio || !rs[2].Audio || rs[2].Status != "waiting" {
		t.Fatalf("kept: %+v %+v", rs[1], rs[2])
	}
}

func TestRedoBeforeEnglish(t *testing.T) {
	ps := newPhotoServer(t)
	x := newTable(t, 6, nil)
	x.play()
	ps.add(x, "LANG")
	for i, clip := range []string{"clip-guessed-1", "clip-english-2", "clip-no-audio-3", "clip-waiting-4"} {
		if c, out := ps.clip("LANG", "host", clip, i+1, x.ps[i], nil, "audio/webm", webm(50)); c != 200 {
			t.Fatalf("upload %d: %d %v", i, c, out)
		}
	}
	db := ps.s.db
	db.Exec(`UPDATE recordings SET transcript='Saya pilih Resilience.', language='ms', transcribed_at=? WHERE clip='clip-guessed-1'`, englishSince-3600)
	db.Exec(`UPDATE recordings SET transcript='I chose Resilience.', language='en', transcribed_at=? WHERE clip='clip-english-2'`, englishSince+60)
	db.Exec(`UPDATE recordings SET transcript='Kept text.', language='ms', transcribed_at=?, audio=NULL WHERE clip='clip-no-audio-3'`, englishSince-3600)

	for range 2 { // once is enough: the second start changes nothing
		ps.s.redoBeforeEnglish()
		rs := ps.list("LANG", "host")
		if r := rs[0]; r.Status != "waiting" || r.Transcript != nil || r.Language != "" || r.TranscribedAt != 0 {
			t.Fatalf("guessed before English: %+v", r)
		}
		if r := rs[1]; r.Status != "done" || *r.Transcript != "I chose Resilience." {
			t.Fatalf("English already: %+v", r)
		}
		if r := rs[2]; r.Transcript == nil || *r.Transcript != "Kept text." {
			t.Fatalf("no audio to redo: %+v", r)
		}
		if r := rs[3]; r.Status != "waiting" {
			t.Fatalf("waiting: %+v", r)
		}
	}
}
