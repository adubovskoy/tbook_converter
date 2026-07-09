// Package embalign runs the local embedding word aligner (tools/embalign.py
// --serve, SimAlign-style: LaBSE token embeddings + mutual argmax) as a child
// process and converts its word-pair output into tbook alignment chunks.
//
// Benchmarked against the production LLM align pass: on lexcheck the
// LaBSE-argmax aligner matches or beats the LLM (support rate 0.710 vs 0.672,
// en→ru) at zero token cost, and it is structurally immune to positional
// drift. The pipeline uses it via AlignMode "emb" or "hybrid".
package embalign

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"sync"

	"github.com/dimando/reader/converter/internal/tbook"
)

// ErrDead marks aligner failures that will fail every subsequent call too
// (broken pipe, process exit) — callers stop instead of retrying per sentence.
var ErrDead = errors.New("embedding aligner process died")

// Options configure the aligner child process.
type Options struct {
	Python string // interpreter; empty = DefaultPython()
	Script string // aligner script; empty = "tools/embalign.py"
	Model  string // HF model id; empty = "sentence-transformers/LaBSE"
	Layer  int    // hidden layer for token embeddings; 0 = 8
	Method string // "argmax" (precision-first, default) or "itermax"
}

func (o *Options) fill() {
	if o.Python == "" {
		o.Python = DefaultPython()
	}
	if o.Script == "" {
		o.Script = "tools/embalign.py"
	}
	if o.Model == "" {
		o.Model = "sentence-transformers/LaBSE"
	}
	if o.Layer == 0 {
		o.Layer = 8
	}
	if o.Method == "" {
		o.Method = "argmax"
	}
}

// DefaultPython resolves the aligner interpreter: $EMBALIGN_PYTHON, then the
// local .venv-embalign (tools/embalign-setup.sh), then python3 from PATH.
func DefaultPython() string {
	if p := os.Getenv("EMBALIGN_PYTHON"); p != "" {
		return p
	}
	if venv := filepath.Join(".venv-embalign", "bin", "python"); isExecutable(venv) {
		return venv
	}
	return "python3"
}

func isExecutable(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && !fi.IsDir() && fi.Mode()&0o111 != 0
}

// Aligner is a handle on the running child process. Align is safe for
// concurrent use (requests are serialized over the single pipe pair).
type Aligner struct {
	mu  sync.Mutex
	cmd *exec.Cmd
	in  io.WriteCloser
	out *bufio.Reader
}

type request struct {
	Src []string `json:"src"`
	Tgt []string `json:"tgt"`
}

type response struct {
	Ready bool     `json:"ready"`
	Pairs [][2]int `json:"pairs"`
	Error string   `json:"error"`
}

// maxLine bounds one response line (pair lists are small; this is headroom).
const maxLine = 4 << 20

// Start launches the aligner and blocks until the model is loaded (the child
// prints a ready line; a cold cache downloads the model first, which can take
// minutes — its progress goes to stderr).
func Start(opts Options) (*Aligner, error) {
	opts.fill()
	if _, err := os.Stat(opts.Script); err != nil {
		return nil, fmt.Errorf("aligner script %s: %w", opts.Script, err)
	}
	cmd := exec.Command(opts.Python, opts.Script, "--serve",
		"--model", opts.Model, "--layer", strconv.Itoa(opts.Layer), "--methods", opts.Method)
	cmd.Stderr = os.Stderr
	in, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	outPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start embedding aligner (%s): %w — run tools/embalign-setup.sh "+
			"or set EMBALIGN_PYTHON", opts.Python, err)
	}
	a := &Aligner{cmd: cmd, in: in, out: bufio.NewReaderSize(outPipe, maxLine)}
	resp, err := a.readResponse()
	if err != nil {
		_ = a.Close()
		return nil, fmt.Errorf("embedding aligner failed to start: %w — run tools/embalign-setup.sh "+
			"or set EMBALIGN_PYTHON", err)
	}
	if !resp.Ready {
		_ = a.Close()
		return nil, fmt.Errorf("embedding aligner handshake: %s", resp.Error)
	}
	return a, nil
}

// Align returns [srcWordIdx, tgtWordIdx] pairs for one sentence pair, given
// the word strings on both sides.
func (a *Aligner) Align(srcWords, tgtWords []string) ([][2]int, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	b, err := json.Marshal(request{Src: srcWords, Tgt: tgtWords})
	if err != nil {
		return nil, err
	}
	if _, err := a.in.Write(append(b, '\n')); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDead, err)
	}
	resp, err := a.readResponse()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDead, err)
	}
	if resp.Error != "" {
		return nil, fmt.Errorf("embedding aligner: %s", resp.Error)
	}
	return resp.Pairs, nil
}

func (a *Aligner) readResponse() (response, error) {
	line, err := a.out.ReadBytes('\n')
	if err != nil {
		return response{}, err
	}
	var resp response
	if err := json.Unmarshal(line, &resp); err != nil {
		return response{}, fmt.Errorf("bad response %q: %w", truncate(line, 200), err)
	}
	return resp, nil
}

func truncate(b []byte, n int) string {
	if len(b) > n {
		return string(b[:n]) + "…"
	}
	return string(b)
}

// Close terminates the child process.
func (a *Aligner) Close() error {
	_ = a.in.Close() // EOF on stdin: the serve loop exits cleanly
	return a.cmd.Wait()
}

// Chunks converts word pairs into alignment chunks — one chunk per aligned
// target word, spanning that word and claiming its source words — plus the
// coverage score Q (fraction of translation words with >=1 source word).
func Chunks(pairs [][2]int, trWords [][2]int) ([]tbook.AlignChunk, float64) {
	byTgt := map[int][]int{}
	for _, pr := range pairs {
		s, t := pr[0], pr[1]
		if t < 0 || t >= len(trWords) || s < 0 {
			continue
		}
		byTgt[t] = append(byTgt[t], s)
	}
	tgts := make([]int, 0, len(byTgt))
	for t := range byTgt {
		tgts = append(tgts, t)
	}
	sort.Ints(tgts)
	chunks := make([]tbook.AlignChunk, 0, len(tgts))
	for _, t := range tgts {
		ws := byTgt[t]
		sort.Ints(ws)
		out := ws[:0]
		for i, w := range ws {
			if i == 0 || w != ws[i-1] {
				out = append(out, w)
			}
		}
		chunks = append(chunks, tbook.AlignChunk{T: trWords[t], W: out})
	}
	q := 0.0
	if len(trWords) > 0 {
		q = float64(len(byTgt)) / float64(len(trWords))
	}
	return chunks, q
}

// WordStrings extracts the word substrings for [start,end) rune offsets.
func WordStrings(text string, words [][2]int) []string {
	runes := []rune(text)
	out := make([]string, 0, len(words))
	for _, w := range words {
		a, b := w[0], w[1]
		if a < 0 || b > len(runes) || a >= b {
			out = append(out, "")
			continue
		}
		out = append(out, string(runes[a:b]))
	}
	return out
}
