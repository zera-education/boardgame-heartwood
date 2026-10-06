package main

// Player photos: each player may take a photo on their phone; the board shows
// it in their round medallion. Photos live in their own table, outside the game
// state, and are deleted soon after the game ends.

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"image"
	_ "image/jpeg" // image.DecodeConfig reads JPEG
	_ "image/png"  // and PNG
	"log"
	"net/http"
	"strings"
	"time"
)

const (
	maxPhotoBytes = 300 * 1024 // decoded; the phone sends a 320×320 JPEG, far smaller
	maxPhotoSide  = 1024
	// The request body: the photo in base64 (4/3 of its size) plus a little JSON.
	maxPhotoBody  = maxPhotoBytes*4/3 + 1024
	photoAfterEnd = 2 * time.Hour  // photos go this long after the result is set
	photoIdle     = 24 * time.Hour // and from games nobody has touched for this long
)

var (
	errNotPhoto     = errors.New("that isn't a photo: send a JPEG or PNG")
	errPhotoTooBig  = errors.New("that photo is too big: 300 KB at most")
	errPhotoTooWide = errors.New("that photo is too big: 1024 pixels a side at most")
)

// decodePhoto turns a data URL ("data:image/jpeg;base64,…") into the image's
// bytes, and checks it is a JPEG or PNG of a sensible size.
func decodePhoto(data string) ([]byte, error) {
	head, b64, ok := strings.Cut(data, ",")
	if !ok || !strings.HasPrefix(head, "data:") || !strings.HasSuffix(head, ";base64") {
		return nil, errNotPhoto
	}
	if base64.StdEncoding.DecodedLen(len(b64)) > maxPhotoBytes+3 {
		return nil, errPhotoTooBig
	}
	b, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, errNotPhoto
	}
	if len(b) > maxPhotoBytes {
		return nil, errPhotoTooBig
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(b))
	if err != nil || (format != "jpeg" && format != "png") || cfg.Width < 1 || cfg.Height < 1 {
		return nil, errNotPhoto
	}
	if cfg.Width > maxPhotoSide || cfg.Height > maxPhotoSide {
		return nil, errPhotoTooWide
	}
	return b, nil
}

// photo stores a player's photo, or removes it when data is "". A player can
// remove their photo at any time, and change it until the game ends.
//
// POST /api/games/{code}/photo {pid, secret, data} → {ok, photo, view}
func (s *Server) photo(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxPhotoBody)
	var req struct {
		Pid    string `json:"pid"`
		Secret string `json:"secret"`
		Data   string `json:"data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			fail(w, 400, errPhotoTooBig.Error())
			return
		}
		fail(w, 400, "bad request")
		return
	}
	var img []byte
	if req.Data != "" {
		var err error
		if img, err = decodePhoto(req.Data); err != nil {
			fail(w, 400, err.Error())
			return
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	g := s.game(w, r)
	if g == nil {
		return
	}
	p := g.player(req.Pid)
	if p == nil || p.Secret != req.Secret {
		fail(w, 403, "not recognised: rejoin the game")
		return
	}
	if img == nil {
		if p.Photo != 0 {
			if err := s.deletePhoto(g.Code, p.ID); err != nil {
				fail(w, 500, err.Error())
				return
			}
			p.Photo = 0
			s.photoChanged(g, p.ID, map[string]any{"removed": true})
		}
	} else {
		if g.Result != "" {
			fail(w, 400, "the game has ended: photos can't be changed now")
			return
		}
		_, err := s.db.Exec(`INSERT INTO photos(code,pid,data) VALUES(?,?,?)
ON CONFLICT(code,pid) DO UPDATE SET data=excluded.data`, g.Code, p.ID, img)
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		// The game version this change gets: a number never used before in this
		// game, so a browser never shows an old cached photo for it.
		p.Photo = g.Version + 1
		s.photoChanged(g, p.ID, map[string]any{"bytes": len(img)})
	}
	writeJSON(w, 200, map[string]any{"ok": true, "photo": p.Photo, "view": buildView(g, p.ID, p.Secret, "")})
}

// photoChanged logs a photo change, then bumps the version and pushes it.
func (s *Server) photoChanged(g *Game, pid string, payload map[string]any) {
	b, _ := json.Marshal(payload)
	s.db.Exec(`INSERT INTO events(code,actor,type,payload,at) VALUES(?,?,?,?,?)`, g.Code, pid, "photo", string(b), time.Now().Unix())
	if err := s.changed(g); err != nil {
		log.Printf("game %s: %v", g.Code, err)
	}
}

// photoImage serves a player's photo. The URL carries ?v=<photo number>, so it
// can be cached: a new photo gets a new number.
//
// GET /api/games/{code}/photo/{pid}?v=N
func (s *Server) photoImage(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	g := s.game(w, r)
	if g == nil {
		return
	}
	var data []byte
	if p := g.player(r.PathValue("pid")); p != nil && p.Photo != 0 {
		s.db.QueryRow(`SELECT data FROM photos WHERE code=? AND pid=?`, g.Code, p.ID).Scan(&data)
	}
	if len(data) == 0 {
		fail(w, 404, "no photo")
		return
	}
	w.Header().Set("Content-Type", http.DetectContentType(data))
	w.Header().Set("Cache-Control", "private, max-age=86400")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Write(data)
}

func (s *Server) deletePhoto(code, pid string) error {
	_, err := s.db.Exec(`DELETE FROM photos WHERE code=? AND pid=?`, code, pid)
	return err
}

// sweepPhotos deletes the photos nobody should keep: those of games whose
// result was set more than 2 hours ago, of games not updated for 24 hours (or
// no longer loaded), and of players who are no longer in their game. Their
// players' photo goes back to 0, and live screens are told. It runs at startup
// and every 10 minutes.
func (s *Server) sweepPhotos(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	type key struct{ code, pid string }
	var gone []key
	rows, err := s.db.Query(`SELECT p.code, p.pid, COALESCE(g.updated_at, 0) FROM photos p LEFT JOIN games g ON g.code = p.code`)
	if err != nil {
		log.Printf("photo sweep: %v", err)
		return
	}
	for rows.Next() {
		var code, pid string
		var updated int64
		if err := rows.Scan(&code, &pid, &updated); err != nil {
			log.Printf("photo sweep: %v", err)
			continue
		}
		g := s.games[code]
		idle := updated < now.Add(-photoIdle).Unix()
		ended := g != nil && g.EndedAt > 0 && g.EndedAt < now.Add(-photoAfterEnd).Unix()
		if g == nil || idle || ended || g.player(pid) == nil {
			gone = append(gone, key{code, pid})
		}
	}
	rows.Close() // before deleting: the database has a single connection
	touched := map[*Game]bool{}
	for _, k := range gone {
		if err := s.deletePhoto(k.code, k.pid); err != nil {
			log.Printf("photo sweep: %v", err)
			continue
		}
		if g := s.games[k.code]; g != nil {
			if p := g.player(k.pid); p != nil && p.Photo != 0 {
				p.Photo = 0
				touched[g] = true
			}
		}
	}
	for g := range touched {
		if err := s.changed(g); err != nil {
			log.Printf("photo sweep: game %s: %v", g.Code, err)
		}
	}
	if len(gone) > 0 {
		log.Printf("photo sweep: deleted %d photos", len(gone))
	}
}
