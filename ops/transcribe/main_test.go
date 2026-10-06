package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// fakeServer plays the Heartwood server's worker API (recording.go) with a few clips.
type fakeServer struct {
	mu      sync.Mutex
	clips   map[int64]string         // id → audio
	results map[int64]map[string]any // what the worker posted
	down    bool
}

func (f *fakeServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.down {
		http.Error(w, "bad gateway", 502)
		return
	}
	if r.Header.Get("Authorization") != "Bearer secret-token" {
		http.Error(w, `{"error":"not the transcription worker"}`, 401)
		return
	}
	var id int64
	switch {
	case r.Method == "GET" && r.URL.Path == "/api/transcribe/queue":
		var items []map[string]any
		for id := range f.clips {
			if _, done := f.results[id]; !done {
				items = append(items, map[string]any{"id": id, "code": "ABCD", "mime": "audio/webm;codecs=opus", "bytes": 5})
			}
		}
		json.NewEncoder(w).Encode(map[string]any{"recordings": items})
	case r.Method == "GET" && sscan(r.URL.Path, "/api/transcribe/%d/audio", &id):
		a, ok := f.clips[id]
		if !ok {
			http.Error(w, `{"error":"no such recording"}`, 404)
			return
		}
		w.Write([]byte(a))
	case r.Method == "POST" && sscan(r.URL.Path, "/api/transcribe/%d", &id):
		if _, ok := f.clips[id]; !ok {
			http.Error(w, `{"error":"no such recording"}`, 404)
			return
		}
		var v map[string]any
		json.NewDecoder(r.Body).Decode(&v)
		f.results[id] = v
		w.Write([]byte(`{"ok":true}`))
	default:
		http.Error(w, "404 page not found", 404)
	}
}

func sscan(path, format string, id *int64) bool {
	n, err := fmt.Sscanf(path, format, id)
	return err == nil && n == 1 && fmt.Sprintf(format, *id) == path
}

func newWorker(t *testing.T, url string, tr Transcriber) (*Worker, *strings.Builder, *strings.Builder) {
	var out, errs strings.Builder
	return &Worker{Server: url, Token: "secret-token", Limit: 20, Budget: time.Minute, StateDir: t.TempDir(),
		Transcribe: tr, Client: &http.Client{Timeout: 5 * time.Second}, Out: &out, Err: &errs}, &out, &errs
}

// fakeWhisper "transcribes" by reading the file: "speech:…" is text, anything else can't be decoded.
func fakeWhisper(ctx context.Context, file string) (Result, error) {
	b, err := os.ReadFile(file)
	if err != nil {
		return Result{}, err
	}
	if !strings.HasSuffix(file, ".webm") {
		return Result{}, fmt.Errorf("wrong extension %s", file)
	}
	if t, ok := strings.CutPrefix(string(b), "speech:"); ok {
		return Result{Text: t, Language: "ms", Duration: 4.5}, nil
	}
	return Result{}, errors.New("transcribe: could not decode")
}

func TestWorkerRun(t *testing.T) {
	f := &fakeServer{clips: map[int64]string{}, results: map[int64]map[string]any{}}
	srv := httptest.NewServer(f)
	defer srv.Close()
	w, out, errs := newWorker(t, srv.URL, fakeWhisper)
	ctx := context.Background()

	// Nothing to do: quiet.
	if code := w.Run(ctx); code != 0 || out.Len() != 0 || errs.Len() != 0 {
		t.Fatalf("empty queue: %d %q %q", code, out, errs)
	}

	// One story, one clip Whisper can't read.
	f.clips[1] = "speech:Saya pilih Resilience kerana nenek saya."
	f.clips[2] = "garbage"
	if code := w.Run(ctx); code != 0 {
		t.Fatalf("run: %d %q", code, errs)
	}
	if r := f.results[1]; r["text"] != "Saya pilih Resilience kerana nenek saya." || r["language"] != "ms" || r["duration"] != 4.5 {
		t.Fatalf("result 1: %v", r)
	}
	if r := f.results[2]; r["error"] != "transcribe: could not decode" || r["text"] != nil {
		t.Fatalf("result 2: %v", r)
	}
	if got := out.String(); !strings.Contains(got, "transcribed 1 story; 1 failed: ABCD#2: transcribe: could not decode") {
		t.Fatalf("summary %q", got)
	}

	// Whisper not set up: nothing is reported (the clip isn't at fault), and the run fails so lanes alerts.
	f.clips[3] = "speech:hello"
	w2, _, errs2 := newWorker(t, srv.URL, func(context.Context, string) (Result, error) {
		return Result{}, fmt.Errorf("%w: no Whisper venv", errSetup)
	})
	if code := w2.Run(ctx); code != 1 || f.results[3] != nil || !strings.Contains(errs2.String(), "no Whisper venv") {
		t.Fatalf("no whisper: %d %v %q", code, f.results[3], errs2)
	}

	// A refused token fails at once.
	w3, _, errs3 := newWorker(t, srv.URL, fakeWhisper)
	w3.Token = "wrong"
	if code := w3.Run(ctx); code != 1 || !strings.Contains(errs3.String(), "refused the token") {
		t.Fatalf("wrong token: %d %q", code, errs3)
	}

	// Unreachable: quiet for 5 runs, a failure on the 6th; one good run resets the count.
	f.down = true
	for k := 1; k <= 6; k++ {
		want := 0
		if k == 6 {
			want = 1
		}
		if code := w.Run(ctx); code != want {
			t.Fatalf("down run %d: %d", k, code)
		}
	}
	f.down = false
	if code := w.Run(ctx); code != 0 || f.results[3]["text"] != "hello" || w.downCount() != 0 {
		t.Fatalf("back up: %d %v %d", code, f.results[3], w.downCount())
	}
	f.down = true
	if code := w.Run(ctx); code != 0 {
		t.Fatalf("one blip after recovering: %d", code)
	}
	f.down = false

	// A server without the API yet (404 on the queue) counts as unreachable, not as a failure.
	plain := httptest.NewServer(http.NotFoundHandler())
	defer plain.Close()
	w4, _, _ := newWorker(t, plain.URL, fakeWhisper)
	if code := w4.Run(ctx); code != 0 || w4.downCount() != 1 {
		t.Fatalf("not deployed yet: %d %d", code, w4.downCount())
	}

	// A run still busy holds the lock: the next one leaves at once.
	f.clips[4] = "speech:later"
	lock, _ := os.OpenFile(filepath.Join(w.StateDir, "transcribe.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	syscall.Flock(int(lock.Fd()), syscall.LOCK_EX)
	if code := w.Run(ctx); code != 0 || f.results[4] != nil {
		t.Fatalf("locked: %d %v", code, f.results[4])
	}
	lock.Close()
	if code := w.Run(ctx); code != 0 || f.results[4]["text"] != "later" {
		t.Fatalf("after the lock: %d %v", code, f.results[4])
	}
}

func TestLastLine(t *testing.T) {
	if got := lastLine("ffmpeg says\n\ntranscribe: could not decode x.webm\n\n"); got != "transcribe: could not decode x.webm" {
		t.Fatalf("%q", got)
	}
	if lastLine(strings.Repeat("x", 400)) != strings.Repeat("x", 300) {
		t.Fatal("not trimmed")
	}
}
