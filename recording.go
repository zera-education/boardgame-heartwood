package main

// Story recordings. The Keeper's laptop records each share (web/recorder.js):
// one clip per share, uploaded when the share closes. A worker on El's Mac mini
// (ops/transcribe) asks for the clips without a transcript, transcribes them
// offline with Steward's Whisper tool and posts the text back. The Keeper
// listens and reads on web/stories.html.
//
// Recordings live in their own table, outside the game state, and are kept:
// they are the team's story record (the photo sweep never touches them). Only
// the audio goes, 30 days after it was transcribed; the transcript stays.

import (
	"bytes"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	maxRecordingBytes  = 25 << 20 // one clip; the recorder sends ~4 KB a second (32 kbit/s Opus)
	recordingAudioKeep = 30 * 24 * time.Hour
	transcribeAttempts = 3 // a clip that fails this often is left alone, until the Keeper asks again
)

// transcribeTokenHash is the SHA-256 (hex) of the transcription worker's bearer
// token. The token itself lives only on the Mini, in
// ~/.config/heartwood/transcribe-token (mode 600). Empty: the worker API is off.
var transcribeTokenHash = "293988cf74ba2e953b66da3b1cfbf5165d6a2fbf7bcb8eecc1bad0a196573f8b"

// recordingsSchema is created with the other tables (openDB).
const recordingsSchema = `
CREATE TABLE IF NOT EXISTS recordings (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  code TEXT NOT NULL,
  clip TEXT NOT NULL UNIQUE,             -- the recorder's id for the clip: a retried upload is stored once
  share_idx INTEGER NOT NULL,
  kind TEXT NOT NULL,                    -- why, ring, harvest, heartwood, treasure
  player_id TEXT NOT NULL DEFAULT '',
  player_name TEXT NOT NULL DEFAULT '',
  player_color TEXT NOT NULL DEFAULT '',
  value_idx INTEGER NOT NULL DEFAULT -1, -- why: the value they stand for; harvest: the tree's value
  prompt TEXT NOT NULL DEFAULT '',
  sub TEXT NOT NULL DEFAULT '',          -- the tagline, the ring number, or the treasure id
  seq INTEGER NOT NULL DEFAULT 0,        -- a treasure's round: answer seq of seq_of
  seq_of INTEGER NOT NULL DEFAULT 0,
  round INTEGER NOT NULL DEFAULT 0,
  partial INTEGER NOT NULL DEFAULT 0,    -- cut short: the Keeper screen closed mid-story
  mime TEXT NOT NULL,
  audio BLOB,                            -- NULL once deleted, 30 days after the transcript
  bytes INTEGER NOT NULL,
  duration_ms INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL,
  transcript TEXT,                       -- NULL: not transcribed yet; '': nothing was said
  language TEXT NOT NULL DEFAULT '',
  transcribed_at INTEGER,
  attempts INTEGER NOT NULL DEFAULT 0,
  error TEXT NOT NULL DEFAULT '',
  audio_deleted_at INTEGER
);
CREATE INDEX IF NOT EXISTS recordings_code ON recordings(code, share_idx);`

var recordedKinds = map[string]bool{"why": true, "ring": true, "harvest": true, "heartwood": true, "treasure": true}

// ---- the game setting ----

// recordAction applies the Keeper's recording settings; ok is false for any
// other action. record: on (n=1) or off (n=0) for the whole game. recordHere:
// the Keeper device (text = its id) that records from now on, so two open
// Keeper screens never both record.
func (g *Game) recordAction(a Action) (ok bool, err error) {
	switch a.Type {
	case "record":
		on := a.N != 0
		if on == !g.NoRecord {
			return true, nil
		}
		g.NoRecord = !on
		if on {
			g.logf("The Keeper switched story recording on.")
		} else {
			g.logf("The Keeper switched story recording off.")
		}
		return true, nil
	case "recordHere":
		if !validID(a.Text) {
			return true, errors.New("that device can't record")
		}
		g.RecDevice = a.Text
		return true, nil
	}
	return false, nil
}

// recordingView adds the setting to a view: phones show whether stories are
// recorded; the Keeper's screens also learn which device records.
func recordingView(g *Game, isHost bool, v map[string]any) {
	v["recording"] = !g.NoRecord
	if isHost {
		v["recDevice"] = g.RecDevice
	}
}

// validID: a clip or device id made by the recorder (8 to 64 letters, digits, - or _).
func validID(s string) bool {
	if len(s) < 8 || len(s) > 64 {
		return false
	}
	for _, c := range s {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
			return false
		}
	}
	return true
}

// ---- audio ----

var errNotAudio = errors.New("that isn't a recording: send WebM, MP4 or Ogg audio")

// audioType checks the upload is audio in a container the recorder makes
// (WebM/Opus from Chrome, MP4/AAC from Safari, Ogg/Opus from Firefox), by its
// Content-Type and its first bytes, and returns the type to store.
func audioType(header string, b []byte) (string, error) {
	base, params, err := mime.ParseMediaType(header)
	if err != nil {
		return "", errNotAudio
	}
	var ok bool
	switch base {
	case "audio/webm", "video/webm":
		base, ok = "audio/webm", len(b) >= 4 && bytes.Equal(b[:4], []byte{0x1a, 0x45, 0xdf, 0xa3}) // EBML
	case "audio/mp4", "video/mp4", "audio/x-m4a", "audio/m4a":
		base, ok = "audio/mp4", len(b) >= 8 && string(b[4:8]) == "ftyp"
	case "audio/ogg", "application/ogg":
		base, ok = "audio/ogg", len(b) >= 4 && string(b[:4]) == "OggS"
	}
	if !ok {
		return "", errNotAudio
	}
	if c := params["codecs"]; c != "" && len(c) <= 40 {
		return mime.FormatMediaType(base, map[string]string{"codecs": c}), nil
	}
	return base, nil
}

func audioExt(ctype string) string {
	switch {
	case strings.HasPrefix(ctype, "audio/mp4"):
		return "m4a"
	case strings.HasPrefix(ctype, "audio/ogg"):
		return "ogg"
	}
	return "webm"
}

func serveAudio(w http.ResponseWriter, r *http.Request, ctype, name string, created int64, audio []byte) {
	w.Header().Set("Content-Type", ctype)
	w.Header().Set("Cache-Control", "private, max-age=3600")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", name+"."+audioExt(ctype)))
	http.ServeContent(w, r, "", time.Unix(created, 0), bytes.NewReader(audio))
}

// ---- the Keeper's API ----

// keeperSecret is the Keeper secret sent with a request: the X-Keeper header,
// or ?host= (an <audio> element can't send headers).
func keeperSecret(r *http.Request) string {
	if h := r.Header.Get("X-Keeper"); h != "" {
		return h
	}
	return r.URL.Query().Get("host")
}

// keeper checks the request comes from the game's Keeper and returns the game
// code. A game that is no longer loaded (saved under older rules) is checked
// against its saved state, so its stories stay reachable.
func (s *Server) keeper(w http.ResponseWriter, r *http.Request) (string, bool) {
	code := strings.ToUpper(r.PathValue("code"))
	s.mu.Lock()
	hostSecret, found := "", false
	if g := s.games[code]; g != nil {
		hostSecret, found = g.HostSecret, true
	}
	s.mu.Unlock()
	if !found {
		var state string
		if err := s.db.QueryRow(`SELECT state FROM games WHERE code=?`, code).Scan(&state); err != nil {
			fail(w, 404, "no game with that code")
			return "", false
		}
		var saved struct {
			HostSecret string `json:"hostSecret"`
		}
		json.Unmarshal([]byte(state), &saved)
		hostSecret = saved.HostSecret
	}
	secret := keeperSecret(r)
	if secret == "" || hostSecret == "" || subtle.ConstantTimeCompare([]byte(secret), []byte(hostSecret)) != 1 {
		fail(w, 403, "only the Keeper can do that")
		return "", false
	}
	return code, true
}

// uploadRecording stores one clip of one share, sent by the Keeper's screen
// when the share closes. The same clip sent again (a retry after a lost
// answer) is stored once.
//
// POST /api/games/{code}/recordings?clip=…&share=N&kind=…&player=…&value=N&prompt=…&sub=…&seq=N&of=N&round=N&ms=N&partial=1
// with the Keeper secret (X-Keeper), and the audio as the body with its Content-Type → {ok, id}
func (s *Server) uploadRecording(w http.ResponseWriter, r *http.Request) {
	code, ok := s.keeper(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	num := func(k string) int { n, _ := strconv.Atoi(q.Get(k)); return n }
	clip, kind, idx := q.Get("clip"), q.Get("kind"), num("share")
	if !validID(clip) || !recordedKinds[kind] || idx < 1 {
		fail(w, 400, "bad recording: it needs a clip id, a share and its kind")
		return
	}
	s.mu.Lock()
	g := s.games[code]
	var name, color string
	known := g != nil && idx <= g.ShareSeq
	if g != nil {
		if p := g.player(q.Get("player")); p != nil {
			name, color = p.Name, p.Color
		}
	}
	s.mu.Unlock()
	if !known {
		fail(w, 404, "no such share in this game")
		return
	}
	if name == "" {
		fail(w, 400, "no such player")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRecordingBytes)
	audio, err := io.ReadAll(r.Body)
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			fail(w, 413, "that recording is too long: 25 MB at most")
			return
		}
		fail(w, 400, "the upload broke off")
		return
	}
	ctype, err := audioType(r.Header.Get("Content-Type"), audio)
	if err != nil {
		fail(w, 415, err.Error())
		return
	}
	text := func(k string, n int) string {
		v := strings.ToValidUTF8(q.Get(k), "")
		if len(v) > n {
			v = v[:n]
		}
		return v
	}
	value := -1
	if v := q.Get("value"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 && n < len(Values) {
			value = n
		}
	}
	ms := max(0, min(num("ms"), 24*3600*1000))
	partial := 0
	if q.Get("partial") == "1" {
		partial = 1
	}
	now := time.Now().Unix()
	res, err := s.db.Exec(`INSERT INTO recordings(code,clip,share_idx,kind,player_id,player_name,player_color,value_idx,prompt,sub,seq,seq_of,round,partial,mime,audio,bytes,duration_ms,created_at)
VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(clip) DO NOTHING`,
		code, clip, idx, kind, q.Get("player"), name, color, value, text("prompt", 1000), text("sub", 200),
		max(0, num("seq")), max(0, num("of")), max(0, num("round")), partial, ctype, audio, len(audio), ms, now)
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	var id int64
	if err := s.db.QueryRow(`SELECT id FROM recordings WHERE clip=?`, clip).Scan(&id); err != nil {
		fail(w, 500, err.Error())
		return
	}
	if n, _ := res.RowsAffected(); n > 0 {
		b, _ := json.Marshal(map[string]any{"id": id, "share": idx, "kind": kind, "bytes": len(audio), "ms": ms})
		s.db.Exec(`INSERT INTO events(code,actor,type,payload,at) VALUES(?,?,?,?,?)`, code, "host", "recording", string(b), now)
	}
	writeJSON(w, 200, map[string]any{"ok": true, "id": id})
}

// Recording is one clip as the stories page sees it (no audio bytes).
type Recording struct {
	ID            int64   `json:"id"`
	Clip          string  `json:"clip"`
	Share         int     `json:"share"`
	Kind          string  `json:"kind"`
	Player        string  `json:"player"`
	PlayerName    string  `json:"playerName"`
	PlayerColor   string  `json:"playerColor"`
	Value         int     `json:"value"`
	Prompt        string  `json:"prompt"`
	Sub           string  `json:"sub"`
	Seq           int     `json:"seq"`
	Of            int     `json:"of"`
	Round         int     `json:"round"`
	Partial       bool    `json:"partial"`
	Mime          string  `json:"mime"`
	Bytes         int     `json:"bytes"`
	DurationMs    int     `json:"durationMs"`
	CreatedAt     int64   `json:"createdAt"`
	Transcript    *string `json:"transcript"` // null until transcribed
	Language      string  `json:"language"`
	TranscribedAt int64   `json:"transcribedAt"`
	Attempts      int     `json:"attempts"`
	Error         string  `json:"error"`
	Audio         bool    `json:"audio"` // false once the audio was deleted
	Status        string  `json:"status"`
}

// status: done (transcribed), failed (gave up after 3 tries), waiting.
func (rec *Recording) status() string {
	switch {
	case rec.Transcript != nil:
		return "done"
	case rec.Attempts >= transcribeAttempts || !rec.Audio:
		return "failed"
	}
	return "waiting"
}

// listRecordings: every clip of the game, in the order the stories were told.
//
// GET /api/games/{code}/recordings (Keeper) → {code, recordings: [...]}
func (s *Server) listRecordings(w http.ResponseWriter, r *http.Request) {
	code, ok := s.keeper(w, r)
	if !ok {
		return
	}
	rows, err := s.db.Query(`SELECT id, clip, share_idx, kind, player_id, player_name, player_color, value_idx, prompt, sub, seq, seq_of,
round, partial, mime, bytes, duration_ms, created_at, transcript, language, COALESCE(transcribed_at, 0), attempts, error, audio IS NOT NULL
FROM recordings WHERE code=? ORDER BY share_idx, created_at, id`, code)
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	defer rows.Close()
	out := []Recording{}
	for rows.Next() {
		var rec Recording
		var transcript sql.NullString
		if err := rows.Scan(&rec.ID, &rec.Clip, &rec.Share, &rec.Kind, &rec.Player, &rec.PlayerName, &rec.PlayerColor, &rec.Value,
			&rec.Prompt, &rec.Sub, &rec.Seq, &rec.Of, &rec.Round, &rec.Partial, &rec.Mime, &rec.Bytes, &rec.DurationMs, &rec.CreatedAt,
			&transcript, &rec.Language, &rec.TranscribedAt, &rec.Attempts, &rec.Error, &rec.Audio); err != nil {
			fail(w, 500, err.Error())
			return
		}
		if transcript.Valid {
			rec.Transcript = &transcript.String
		}
		rec.Status = rec.status()
		out = append(out, rec)
	}
	writeJSON(w, 200, map[string]any{"code": code, "recordings": out})
}

func recordingID(r *http.Request) int64 {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	return id
}

// recordingAudio serves one clip (with Range, so the player can seek).
//
// GET /api/games/{code}/recordings/{id}/audio?host=…
func (s *Server) recordingAudio(w http.ResponseWriter, r *http.Request) {
	code, ok := s.keeper(w, r)
	if !ok {
		return
	}
	var audio []byte
	var ctype string
	var share int
	var created int64
	err := s.db.QueryRow(`SELECT audio, mime, share_idx, created_at FROM recordings WHERE id=? AND code=?`, recordingID(r), code).
		Scan(&audio, &ctype, &share, &created)
	if err != nil {
		fail(w, 404, "no such recording")
		return
	}
	if audio == nil {
		fail(w, 404, "the audio was deleted 30 days after it was transcribed; the transcript is kept")
		return
	}
	serveAudio(w, r, ctype, fmt.Sprintf("%s-story-%d", code, share), created, audio)
}

// deleteRecording drops one clip, audio and transcript ("Don't keep this one").
//
// DELETE /api/games/{code}/recordings/{id} (Keeper) → {ok}
func (s *Server) deleteRecording(w http.ResponseWriter, r *http.Request) {
	code, ok := s.keeper(w, r)
	if !ok {
		return
	}
	id := recordingID(r)
	res, err := s.db.Exec(`DELETE FROM recordings WHERE id=? AND code=?`, id, code)
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		fail(w, 404, "no such recording")
		return
	}
	s.db.Exec(`INSERT INTO events(code,actor,type,payload,at) VALUES(?,?,?,?,?)`, code, "host", "deleteRecording", fmt.Sprintf(`{"id":%d}`, id), time.Now().Unix())
	writeJSON(w, 200, map[string]any{"ok": true})
}

// retryRecording asks for one more try at a clip that couldn't be transcribed.
//
// POST /api/games/{code}/recordings/{id}/retry (Keeper) → {ok}
func (s *Server) retryRecording(w http.ResponseWriter, r *http.Request) {
	code, ok := s.keeper(w, r)
	if !ok {
		return
	}
	res, err := s.db.Exec(`UPDATE recordings SET attempts=0, error='' WHERE id=? AND code=? AND transcript IS NULL AND audio IS NOT NULL`, recordingID(r), code)
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		fail(w, 404, "nothing to try again")
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

// ---- the transcription worker's API (bearer token) ----

func (s *Server) worker(w http.ResponseWriter, r *http.Request) bool {
	token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if transcribeTokenHash == "" {
		fail(w, 503, "transcription is off on this server")
		return false
	}
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	if !ok || subtle.ConstantTimeCompare([]byte(hex.EncodeToString(sum[:])), []byte(transcribeTokenHash)) != 1 {
		fail(w, 401, "not the transcription worker")
		return false
	}
	return true
}

// transcribeQueue lists the clips waiting for a transcript, oldest first.
//
// GET /api/transcribe/queue?limit=N → {recordings: [{id, code, mime, bytes, durationMs, attempts}]}
func (s *Server) transcribeQueue(w http.ResponseWriter, r *http.Request) {
	if !s.worker(w, r) {
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 || limit > 100 {
		limit = 20
	}
	rows, err := s.db.Query(`SELECT id, code, mime, bytes, duration_ms, attempts FROM recordings
WHERE transcript IS NULL AND audio IS NOT NULL AND attempts < ? ORDER BY id LIMIT ?`, transcribeAttempts, limit)
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	defer rows.Close()
	type item struct {
		ID         int64  `json:"id"`
		Code       string `json:"code"`
		Mime       string `json:"mime"`
		Bytes      int    `json:"bytes"`
		DurationMs int    `json:"durationMs"`
		Attempts   int    `json:"attempts"`
	}
	out := []item{}
	for rows.Next() {
		var it item
		if err := rows.Scan(&it.ID, &it.Code, &it.Mime, &it.Bytes, &it.DurationMs, &it.Attempts); err != nil {
			fail(w, 500, err.Error())
			return
		}
		out = append(out, it)
	}
	writeJSON(w, 200, map[string]any{"recordings": out})
}

// transcribeAudio hands the worker one clip.
//
// GET /api/transcribe/{id}/audio
func (s *Server) transcribeAudio(w http.ResponseWriter, r *http.Request) {
	if !s.worker(w, r) {
		return
	}
	var audio []byte
	var ctype, code string
	var created int64
	err := s.db.QueryRow(`SELECT audio, mime, code, created_at FROM recordings WHERE id=?`, recordingID(r)).Scan(&audio, &ctype, &code, &created)
	if err != nil || audio == nil {
		fail(w, 404, "no such recording")
		return
	}
	serveAudio(w, r, ctype, fmt.Sprintf("%s-%d", code, recordingID(r)), created, audio)
}

// transcribed stores the worker's result: the text (or "" when nothing was
// said), or an error, which counts as a try; after 3 the clip is left alone.
//
// POST /api/transcribe/{id} {text, language, duration} | {error} → {ok, attempts}
func (s *Server) transcribed(w http.ResponseWriter, r *http.Request) {
	if !s.worker(w, r) {
		return
	}
	var req struct {
		Text     *string `json:"text"`
		Language string  `json:"language"`
		Duration float64 `json:"duration"` // seconds
		Error    string  `json:"error"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || (req.Text == nil && req.Error == "") {
		fail(w, 400, "send {text, language, duration} or {error}")
		return
	}
	id := recordingID(r)
	var res sql.Result
	var err error
	if req.Error != "" {
		msg := strings.ToValidUTF8(req.Error, "")
		if len(msg) > 500 {
			msg = msg[:500]
		}
		res, err = s.db.Exec(`UPDATE recordings SET attempts=attempts+1, error=? WHERE id=? AND transcript IS NULL`, msg, id)
	} else {
		lang := req.Language
		if len(lang) > 16 {
			lang = lang[:16]
		}
		res, err = s.db.Exec(`UPDATE recordings SET transcript=?, language=?, transcribed_at=?, error='',
duration_ms=CASE WHEN ? > 0 THEN ? ELSE duration_ms END WHERE id=?`,
			strings.TrimSpace(strings.ToValidUTF8(*req.Text, "")), lang, time.Now().Unix(), req.Duration, int(req.Duration*1000), id)
	}
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		fail(w, 404, "no such recording waiting for a transcript")
		return
	}
	var attempts int
	s.db.QueryRow(`SELECT attempts FROM recordings WHERE id=?`, id).Scan(&attempts)
	writeJSON(w, 200, map[string]any{"ok": true, "attempts": attempts})
}

// sweepRecordings deletes the audio of stories transcribed more than 30 days
// ago. The transcript and the rest of the record stay. It runs with the photo
// sweep (at startup and every 10 minutes).
func (s *Server) sweepRecordings(now time.Time) {
	res, err := s.db.Exec(`UPDATE recordings SET audio=NULL, audio_deleted_at=? WHERE audio IS NOT NULL AND transcript IS NOT NULL AND transcribed_at < ?`,
		now.Unix(), now.Add(-recordingAudioKeep).Unix())
	if err != nil {
		log.Printf("recording sweep: %v", err)
		return
	}
	if n, _ := res.RowsAffected(); n > 0 {
		log.Printf("recording sweep: deleted the audio of %d transcribed stories", n)
	}
}

func (s *Server) recordingRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/games/{code}/recordings", s.uploadRecording)
	mux.HandleFunc("GET /api/games/{code}/recordings", s.listRecordings)
	mux.HandleFunc("GET /api/games/{code}/recordings/{id}/audio", s.recordingAudio)
	mux.HandleFunc("DELETE /api/games/{code}/recordings/{id}", s.deleteRecording)
	mux.HandleFunc("POST /api/games/{code}/recordings/{id}/retry", s.retryRecording)
	mux.HandleFunc("GET /api/transcribe/queue", s.transcribeQueue)
	mux.HandleFunc("GET /api/transcribe/{id}/audio", s.transcribeAudio)
	mux.HandleFunc("POST /api/transcribe/{id}", s.transcribed)
}
