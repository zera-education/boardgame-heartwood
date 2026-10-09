// Command hw-transcribe turns Heartwood's story recordings into text, on El's Mac mini.
//
// The Keeper's laptop records each share and uploads it to the Heartwood server (recording.go). Every 5 minutes
// (the lanes job heartwood/stories/transcribe) this asks the server for the clips without a transcript, downloads
// each one, transcribes it offline with Steward's Whisper tool (steward run transcribe text) and posts the text back.
// Every story comes out in English: the team speaks Malaysian English, which Whisper's own guess often took for Malay
// (it then wrote the English it heard in Malay); a story told in Malay or Chinese is translated. Audio travels only
// between the Heartwood server and the Mini. With nothing to do it says nothing.
//
// The Mini keeps its own copy of every story it transcribes, with what the story was (game, round, player, question):
// one JSON file per story in ~/.local/share/heartwood/stories/<CODE>/ (-archive), for game reports.
//
//	hw-transcribe [-server URL] [-token FILE] [-limit N] [-budget 4m] [-archive DIR]
//
// The server is $HW_SERVER or https://heartwood.zera.edu.my; the token is the one line in
// ~/.config/heartwood/transcribe-token (mode 600; the server knows only its SHA-256).
//
// Exit 0 when it did its work or had none. A clip Whisper can't read is reported to the server (after 3 tries it is
// left alone until the Keeper asks again on the stories page). Exit 1, so lanes alerts El once, when Whisper isn't
// set up, the token is refused, or the server has been unreachable for 6 runs in a row (30 minutes).
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	defaultServer = "https://heartwood.zera.edu.my"
	clipTimeout   = 10 * time.Minute // a clip is 20 minutes at most; Whisper runs ~20× faster than real time
	downAlert     = 6                // unreachable runs in a row before it counts as a failure (30 minutes)
	userAgent     = "heartwood-transcribe/1"
)

// Result is what Steward's transcribe tool writes with --out (the fields used here).
type Result struct {
	Text     string  `json:"text"`
	Language string  `json:"language"`
	Duration float64 `json:"duration"`
}

// errSetup: Whisper (venv, model or ffmpeg) is missing on this machine. Nothing is reported to the server: the clip
// isn't at fault.
var errSetup = errors.New("whisper is not set up")

// Transcriber turns one audio file into text.
type Transcriber func(ctx context.Context, file string) (Result, error)

// stewardTranscriber runs `steward run transcribe text <file> --lang en --out <json>`. Exit codes (tools/transcribe): 2 the venv,
// model or ffmpeg is missing · 3 the audio could not be decoded · 4 bad input · 5 bug.
func stewardTranscriber(steward string) Transcriber {
	return func(ctx context.Context, file string) (Result, error) {
		out := file + ".json"
		defer os.Remove(out)
		cmd := exec.CommandContext(ctx, steward, "run", "transcribe", "text", file, "--lang", "en", "--out", out)
		var stderr strings.Builder
		cmd.Stdout, cmd.Stderr = io.Discard, &stderr
		err := cmd.Run()
		if ctx.Err() != nil {
			return Result{}, fmt.Errorf("transcription took longer than %s", clipTimeout)
		}
		if err != nil {
			msg := lastLine(stderr.String())
			var ee *exec.ExitError
			if errors.As(err, &ee) && ee.ExitCode() == 2 {
				return Result{}, fmt.Errorf("%w: %s", errSetup, msg)
			}
			if msg == "" {
				msg = err.Error()
			}
			return Result{}, errors.New(msg)
		}
		b, err := os.ReadFile(out)
		if err != nil {
			return Result{}, fmt.Errorf("no result file: %v", err)
		}
		var r Result
		if err := json.Unmarshal(b, &r); err != nil {
			return Result{}, fmt.Errorf("unreadable result: %v", err)
		}
		return r, nil
	}
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if l := strings.TrimSpace(lines[i]); l != "" {
			if len(l) > 300 {
				l = l[:300]
			}
			return l
		}
	}
	return ""
}

// Worker is one run against one server.
type Worker struct {
	Server     string
	Token      string
	Limit      int
	Budget     time.Duration // start no new clip after this long; the next run takes the rest
	StateDir   string        // the lock and the count of unreachable runs
	Archive    string        // the Mini's copy of each story ("": none)
	Transcribe Transcriber
	Client     *http.Client
	Out, Err   io.Writer
}

type queued struct {
	ID         int64  `json:"id"`
	Code       string `json:"code"`
	Mime       string `json:"mime"`
	Bytes      int    `json:"bytes"`
	DurationMs int    `json:"durationMs"`
	Attempts   int    `json:"attempts"`
	// what the story was
	Share       int    `json:"share"`
	Kind        string `json:"kind"`
	PlayerName  string `json:"playerName"`
	PlayerColor string `json:"playerColor"`
	Value       int    `json:"value"`
	Prompt      string `json:"prompt"`
	Sub         string `json:"sub"`
	Seq         int    `json:"seq"`
	Of          int    `json:"of"`
	Round       int    `json:"round"`
	CreatedAt   int64  `json:"createdAt"`
}

// errUnreachable: the server didn't answer properly (network, 5xx, not deployed yet). Counted, not alerted at once.
type errUnreachable struct{ error }

// errStatus: the server answered with an error status.
type errStatus struct {
	status int
	msg    string
}

func (e errStatus) Error() string { return e.msg }

func (w *Worker) request(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(w.Server, "/")+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+w.Token)
	req.Header.Set("User-Agent", userAgent)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := w.Client.Do(req)
	if err != nil {
		return nil, errUnreachable{err}
	}
	if resp.StatusCode == 401 {
		resp.Body.Close()
		return nil, errors.New("the server refused the token (check ~/.config/heartwood/transcribe-token against transcribeTokenHash in recording.go)")
	}
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 300))
		resp.Body.Close()
		e := errStatus{resp.StatusCode, fmt.Sprintf("%s %s: %d %s", method, path, resp.StatusCode, strings.TrimSpace(string(b)))}
		if resp.StatusCode >= 500 || (strings.HasPrefix(path, "/api/transcribe/queue") && resp.StatusCode == 404) {
			return nil, errUnreachable{e}
		}
		return nil, e
	}
	return resp, nil
}

func (w *Worker) queue(ctx context.Context) ([]queued, error) {
	resp, err := w.request(ctx, "GET", fmt.Sprintf("/api/transcribe/queue?limit=%d", w.Limit), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out struct {
		Recordings []queued `json:"recordings"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, errUnreachable{fmt.Errorf("the queue: %v", err)}
	}
	return out.Recordings, nil
}

func (w *Worker) download(ctx context.Context, q queued, dir string) (string, error) {
	resp, err := w.request(ctx, "GET", fmt.Sprintf("/api/transcribe/%d/audio", q.ID), nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	ext := ".webm"
	switch {
	case strings.HasPrefix(q.Mime, "audio/mp4"):
		ext = ".m4a"
	case strings.HasPrefix(q.Mime, "audio/ogg"):
		ext = ".ogg"
	}
	file := filepath.Join(dir, fmt.Sprintf("%s-%d%s", q.Code, q.ID, ext))
	f, err := os.Create(file)
	if err != nil {
		return "", err
	}
	_, err = io.Copy(f, io.LimitReader(resp.Body, 30<<20))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return "", errUnreachable{fmt.Errorf("downloading %s#%d: %v", q.Code, q.ID, err)}
	}
	return file, nil
}

func (w *Worker) report(ctx context.Context, id int64, v any) error {
	b, _ := json.Marshal(v)
	resp, err := w.request(ctx, "POST", fmt.Sprintf("/api/transcribe/%d", id), strings.NewReader(string(b)))
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// Run does one pass and returns the exit code.
func (w *Worker) Run(ctx context.Context) int {
	if err := os.MkdirAll(w.StateDir, 0o700); err != nil {
		fmt.Fprintln(w.Err, "hw-transcribe:", err)
		return 1
	}
	// One run at a time: a long story may outlast the 5 minutes to the next run.
	lock, err := os.OpenFile(filepath.Join(w.StateDir, "transcribe.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		fmt.Fprintln(w.Err, "hw-transcribe:", err)
		return 1
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return 0 // the previous run is still busy
	}

	start := time.Now()
	items, err := w.queue(ctx)
	if err != nil {
		return w.failed(err)
	}
	w.reachable()
	if len(items) == 0 {
		return 0
	}
	dir, err := os.MkdirTemp("", "hw-transcribe-")
	if err != nil {
		fmt.Fprintln(w.Err, "hw-transcribe:", err)
		return 1
	}
	defer os.RemoveAll(dir)

	done, failed, skipped := 0, []string{}, 0
	for _, q := range items {
		if time.Since(start) > w.Budget {
			break
		}
		name := fmt.Sprintf("%s#%d", q.Code, q.ID)
		file, err := w.download(ctx, q, dir)
		if gone(err) { // the Keeper deleted it meanwhile
			skipped++
			continue
		}
		if err != nil {
			return w.failed(err)
		}
		cctx, cancel := context.WithTimeout(ctx, clipTimeout)
		res, terr := w.Transcribe(cctx, file)
		cancel()
		os.Remove(file)
		if errors.Is(terr, errSetup) {
			fmt.Fprintln(w.Err, "hw-transcribe:", terr, "(see ~/steward/tools/transcribe/README.md, Setup)")
			return 1
		}
		if terr != nil {
			err = w.report(ctx, q.ID, map[string]string{"error": terr.Error()})
		} else {
			res.Text = tidy(res.Text)
			err = w.report(ctx, q.ID, map[string]any{"text": res.Text, "language": res.Language, "duration": res.Duration})
			if err == nil {
				w.keep(q, res)
			}
		}
		switch {
		case gone(err):
			skipped++
		case err != nil:
			return w.failed(err)
		case terr != nil:
			failed = append(failed, name+": "+terr.Error())
		default:
			done++
		}
	}
	msg := fmt.Sprintf("transcribed %d %s", done, plural(done, "story", "stories"))
	if len(failed) > 0 {
		msg += fmt.Sprintf("; %d failed: %s", len(failed), strings.Join(failed, "; "))
	}
	if skipped > 0 {
		msg += fmt.Sprintf("; %d deleted meanwhile", skipped)
	}
	if left := len(items) - done - len(failed) - skipped; left > 0 {
		msg += fmt.Sprintf("; %d left for the next run", left)
	}
	fmt.Fprintln(w.Out, msg)
	return 0
}

// keep writes the Mini's copy of a story: <archive>/<CODE>/<share>-<id>.json. A story transcribed again replaces
// its file. A failure here is only said: the transcript is on the server.
func (w *Worker) keep(q queued, res Result) {
	if w.Archive == "" || !validCode(q.Code) {
		return
	}
	dir := filepath.Join(w.Archive, q.Code)
	b, _ := json.MarshalIndent(map[string]any{
		"id": q.ID, "code": q.Code, "share": q.Share, "round": q.Round, "kind": q.Kind, "playerName": q.PlayerName,
		"playerColor": q.PlayerColor, "value": q.Value, "prompt": q.Prompt, "sub": q.Sub, "seq": q.Seq, "of": q.Of,
		"durationMs": q.DurationMs, "createdAt": q.CreatedAt, "text": res.Text, "language": res.Language,
		"transcribedAt": time.Now().Unix(),
	}, "", "  ")
	err := os.MkdirAll(dir, 0o700)
	if err == nil {
		err = os.WriteFile(filepath.Join(dir, fmt.Sprintf("%03d-%d.json", q.Share, q.ID)), b, 0o600)
	}
	if err != nil {
		fmt.Fprintln(w.Err, "hw-transcribe: keeping a copy:", err)
	}
}

// validCode: a game code is letters and digits (it names a folder).
func validCode(s string) bool {
	if s == "" || len(s) > 12 {
		return false
	}
	for _, c := range s {
		if !(c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9') {
			return false
		}
	}
	return true
}

// tidy takes out Whisper's loops. On noisy audio it can write one word or a short phrase dozens of times
// ("oh, oh, oh, …"): four or more of the same 1 to 4 words in a row keep one. A long "word" of two or three
// letters over and over ("SASASASA…") goes.
func tidy(text string) string {
	var paras []string
	for _, p := range strings.Split(text, "\n\n") {
		if p = tidyPara(p); p != "" {
			paras = append(paras, p)
		}
	}
	return strings.Join(paras, "\n\n")
}

func tidyPara(p string) string {
	var words []string
	for _, f := range strings.Fields(p) {
		if n := norm(f); utf8.RuneCountInString(n) >= 20 && distinct(n) <= 3 {
			continue
		}
		words = append(words, f)
	}
	same := func(i, j, k int) bool { // words[i:i+k] reads as words[j:j+k]
		for x := 0; x < k; x++ {
			a, b := norm(words[i+x]), norm(words[j+x])
			if a == "" || a != b {
				return false
			}
		}
		return true
	}
	var out []string
	for i := 0; i < len(words); {
		bestK, bestN := 0, 0
		for k := 1; k <= 4 && i+k <= len(words); k++ {
			n := 1
			for i+(n+1)*k <= len(words) && same(i, i+n*k, k) {
				n++
			}
			if n >= 4 && n*k > bestN*bestK {
				bestK, bestN = k, n
			}
		}
		if bestK == 0 {
			out = append(out, words[i])
			i++
			continue
		}
		last := i + (bestN-1)*bestK // the last time round keeps its punctuation
		out = append(out, words[last:last+bestK]...)
		i += bestN * bestK
	}
	return strings.Join(out, " ")
}

// norm: a word without its punctuation, in lower case.
func norm(w string) string {
	return strings.ToLower(strings.TrimFunc(w, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }))
}

func distinct(s string) int {
	seen := map[rune]bool{}
	for _, r := range s {
		seen[r] = true
	}
	return len(seen)
}

// gone: the clip was deleted (or transcribed elsewhere) while this run had it.
func gone(err error) bool {
	var st errStatus
	return errors.As(err, &st) && st.status == 404
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// failed decides the exit code for an error talking to the server: an unreachable server fails only after 6 runs
// in a row, so a deploy or a Wi-Fi blip doesn't page El.
func (w *Worker) failed(err error) int {
	var down errUnreachable
	if !errors.As(err, &down) {
		fmt.Fprintln(w.Err, "hw-transcribe:", err)
		return 1
	}
	n := w.downCount() + 1
	os.WriteFile(filepath.Join(w.StateDir, "unreachable"), []byte(fmt.Sprint(n)), 0o600)
	fmt.Fprintf(w.Err, "hw-transcribe: %s unreachable (%d in a row): %v\n", w.Server, n, down.error)
	if n >= downAlert {
		return 1
	}
	return 0
}

func (w *Worker) downCount() int {
	b, _ := os.ReadFile(filepath.Join(w.StateDir, "unreachable"))
	var n int
	fmt.Sscan(string(b), &n)
	return n
}

func (w *Worker) reachable() {
	if w.downCount() > 0 {
		os.Remove(filepath.Join(w.StateDir, "unreachable"))
	}
}

func main() {
	home, _ := os.UserHomeDir()
	server := os.Getenv("HW_SERVER")
	if server == "" {
		server = defaultServer
	}
	steward, err := exec.LookPath("steward")
	if err != nil {
		steward = filepath.Join(home, ".local/bin/steward")
	}
	flag.StringVar(&server, "server", server, "the Heartwood server")
	tokenFile := flag.String("token", filepath.Join(home, ".config/heartwood/transcribe-token"), "file holding the worker's bearer token")
	flag.StringVar(&steward, "steward", steward, "the steward command")
	limit := flag.Int("limit", 100, "clips to fetch per run (the budget ends a run sooner)")
	budget := flag.Duration("budget", 4*time.Minute, "start no new clip after this long")
	stateDir := flag.String("state", filepath.Join(home, ".cache/heartwood"), "lock and state directory")
	archive := flag.String("archive", filepath.Join(home, ".local/share/heartwood/stories"), `the Mini's copy of each story ("": none)`)
	flag.Parse()

	tok, err := os.ReadFile(*tokenFile)
	if err != nil || strings.TrimSpace(string(tok)) == "" {
		fmt.Fprintf(os.Stderr, "hw-transcribe: no token in %s (ops/transcribe/install.sh makes one)\n", *tokenFile)
		os.Exit(1)
	}
	w := &Worker{
		Server: server, Token: strings.TrimSpace(string(tok)), Limit: *limit, Budget: *budget, StateDir: *stateDir, Archive: *archive,
		Transcribe: stewardTranscriber(steward), Client: &http.Client{Timeout: 3 * time.Minute}, Out: os.Stdout, Err: os.Stderr,
	}
	os.Exit(w.Run(context.Background()))
}
