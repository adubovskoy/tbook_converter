package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// The --summarize mode: re-render the report tables from artifacts already on
// disk, without sending a request. Probe runs are expensive and the gateway's
// availability changes by the hour, so the tables in the report must be
// derivable from the saved rows alone — this is what makes them reproducible.
//
//	gonka-probe --summarize bench-quality/.artifacts/gonka-probe-*.json \
//	            bench-quality/.artifacts/gonka-smoke-*/  *.jsonl
//
// A .json argument is read as a probe artifact (served list, cells, variants);
// a .jsonl argument, or a directory of them, is read as a dump of returned
// translations and scanned for the defects the smoke read looks for; a
// .raw.jsonl argument is read as a capture of raw replies and scanned for the
// ones the translate parser rejects.

// dumpScan is the script-sanity read of one {src,tr} JSONL dump.
type dumpScan struct {
	Path      string
	Lines     int
	NonTarget int // translation is not majority target script
	Leaks     int // Latin-script words absent from the source
	Identical int // translation byte-identical to the source
	Empty     int
	NoFinal   int // no sentence-final punctuation though the source has it
	Examples  []dumpFinding
}

// dumpFinding is one flagged line, kept verbatim for quoting in the report.
type dumpFinding struct {
	Kind  string
	Src   string
	Tr    string
	Leaks []string
}

// rawScan is the parse read of one raw-reply capture: which replies the
// translate parser would reject, kept verbatim so the report can quote them.
type rawScan struct {
	Path     string
	Replies  int
	Lenient  int // rejected by a strict parse, recovered by the tolerant one
	Bad      int // not usable at all: the batch is lost
	Findings []rawFinding
}

type rawFinding struct {
	N         int
	Chars     int
	Reason    string
	Recovered bool // a lenient extraction would still have found the object
	Content   string
}

// summarizeFiles is the mode's entry point. It prints, in order: the served
// model list each artifact saw, the cell table, the variant table, and the
// dump scan.
func summarizeFiles(paths []string) error {
	arts, dumps, raws, err := collectInputs(paths)
	if err != nil {
		return err
	}
	if len(arts) == 0 && len(dumps) == 0 && len(raws) == 0 {
		return fmt.Errorf("no artifacts or dumps among %d path(s)", len(paths))
	}

	if len(arts) > 0 {
		fmt.Print(renderServedTable(arts))
		var cells []cellSummary
		var runs []string
		var variants []variantResult
		for _, a := range arts {
			for _, c := range a.art.Cells {
				if c.BatchSize == 0 {
					c.BatchSize = a.art.BatchSize
				}
				if c.Run == "" {
					c.Run = a.art.Run
				}
				cells = append(cells, c)
				runs = append(runs, a.art.StartedAt)
			}
			variants = append(variants, a.art.Variants...)
		}
		if len(cells) > 0 {
			fmt.Print(renderCellTable(cells, runs))
		}
		if len(variants) > 0 {
			fmt.Print(renderVariantTable(variants))
		}
	}
	if len(dumps) > 0 {
		fmt.Print(renderDumpTable(dumps))
	}
	if len(raws) > 0 {
		fmt.Print(renderRawTable(raws))
	}
	return nil
}

// loadedArtifact pairs a parsed artifact with the file it came from.
type loadedArtifact struct {
	path string
	art  artifact
}

// collectInputs sorts the arguments into artifacts and dumps, expanding any
// directory into the .json/.jsonl files directly inside it.
func collectInputs(paths []string) ([]loadedArtifact, []dumpScan, []rawScan, error) {
	var files []string
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			return nil, nil, nil, err
		}
		if !info.IsDir() {
			files = append(files, p)
			continue
		}
		entries, err := os.ReadDir(p)
		if err != nil {
			return nil, nil, nil, err
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			switch filepath.Ext(e.Name()) {
			case ".json", ".jsonl":
				files = append(files, filepath.Join(p, e.Name()))
			}
		}
	}
	var arts []loadedArtifact
	var dumps []dumpScan
	var raws []rawScan
	for _, f := range files {
		if strings.HasSuffix(f, ".raw.jsonl") {
			s, err := scanRaw(f)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("%s: %w", f, err)
			}
			raws = append(raws, s)
			continue
		}
		switch filepath.Ext(f) {
		case ".jsonl":
			s, err := scanDump(f)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("%s: %w", f, err)
			}
			dumps = append(dumps, s)
		case ".json":
			a, err := loadArtifact(f)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("%s: %w", f, err)
			}
			arts = append(arts, loadedArtifact{path: f, art: a})
		default:
			return nil, nil, nil, fmt.Errorf("%s: not a .json artifact or .jsonl dump", f)
		}
	}
	return arts, dumps, raws, nil
}

func loadArtifact(path string) (artifact, error) {
	var a artifact
	b, err := os.ReadFile(path)
	if err != nil {
		return a, err
	}
	if err := json.Unmarshal(b, &a); err != nil {
		return a, err
	}
	if a.Tool != "" && a.Tool != "gonka-probe" {
		return a, fmt.Errorf("tool = %q, not a gonka-probe artifact", a.Tool)
	}
	return a, nil
}

// scanDump reads one {src,tr} JSONL and flags the lines a smoke read cares
// about. It keeps every flagged line verbatim: the report quotes broken
// output, and a paraphrase is not evidence.
func scanDump(path string) (dumpScan, error) {
	s := dumpScan{Path: path}
	f, err := os.Open(path)
	if err != nil {
		return s, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var r dumpRow
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			return s, err
		}
		s.Lines++
		src, tr := strings.TrimSpace(r.Src), strings.TrimSpace(r.Tr)
		add := func(kind string, leaks []string) {
			if len(s.Examples) < 40 {
				s.Examples = append(s.Examples, dumpFinding{Kind: kind, Src: src, Tr: tr, Leaks: leaks})
			}
		}
		switch {
		case tr == "":
			s.Empty++
			add("empty", nil)
		case tr == src:
			s.Identical++
			add("identical to source", nil)
		}
		if tr == "" {
			continue
		}
		if !cyrillicMajority(tr) {
			s.NonTarget++
			add("not majority target script", nil)
		}
		if leaks := latinLeaks(src, tr); len(leaks) > 0 {
			s.Leaks++
			add("latin leak", leaks)
		}
		if endsSentence(src) && !endsSentence(tr) {
			s.NoFinal++
			add("no sentence-final punctuation", nil)
		}
	}
	return s, sc.Err()
}

// endsSentence reports whether a string ends the way a finished sentence does.
// A translation that stops mid-clause is the only truncation visible in a dump:
// a response cut by the output cap never reaches the file at all.
func endsSentence(s string) bool {
	s = strings.TrimRight(strings.TrimSpace(s), "»\"'”’)]")
	if s == "" {
		return false
	}
	r := []rune(s)
	switch r[len(r)-1] {
	case '.', '!', '?', ':', ';', '\u2026', // Latin
		'\u3002', '\uff01', '\uff1f': // CJK full stop, exclamation, question
		return true
	}
	return false
}

func renderServedTable(arts []loadedArtifact) string {
	var sb strings.Builder
	sb.WriteString("\n| artifact | run | started (UTC) | listed by /v1/models | probed ids not listed |\n")
	sb.WriteString("|---|---|---|---|---|\n")
	for _, a := range arts {
		var listed, unlisted []string
		for _, m := range a.art.Models {
			if m.Listed {
				listed = append(listed, "`"+m.ID+"`")
			} else if m.Probed {
				unlisted = append(unlisted, "`"+m.ID+"`")
			}
		}
		sort.Strings(listed)
		sort.Strings(unlisted)
		fmt.Fprintf(&sb, "| `%s` | `%s` | %s | %s | %s |\n",
			filepath.Base(a.path), a.art.Run, dash(a.art.StartedAt), joinOrDash(listed), joinOrDash(unlisted))
	}
	return sb.String()
}

func renderCellTable(cells []cellSummary, started []string) string {
	var sb strings.Builder
	sb.WriteString("\n| model | batch | conc | run | started (UTC) | req | ok % | 429 % | 502 % | timeout/other % | trunc % | parse % | p50 s | p90 s | sent/s | sent | out tok/req | cyr % | latin % |\n")
	sb.WriteString("|---|--:|--:|---|---|--:|--:|--:|--:|--:|--:|--:|--:|--:|--:|--:|--:|--:|--:|\n")
	for i, c := range cells {
		ts := ""
		if i < len(started) {
			ts = started[i]
		}
		fmt.Fprintf(&sb, "| `%s` | %d | %d | `%s` | %s | %d | %.1f | %.1f | %.1f | %.1f | %.1f | %.1f | %.1f | %.1f | %.2f | %d/%d | %d | %.0f | %.0f |\n",
			c.Model, c.BatchSize, c.Concurrency, c.Run, dash(ts), c.Requests,
			c.SuccessPct, c.Pct429, c.Pct502, c.PctOther, c.PctTrunc, c.PctParse,
			float64(c.P50MS)/1000, float64(c.P90MS)/1000, c.SentPerSec,
			c.SentGot, c.SentAsked, c.MeanOutTok, c.CyrPct, c.LatinPct)
	}
	return sb.String()
}

func renderDumpTable(dumps []dumpScan) string {
	var sb strings.Builder
	sb.WriteString("\n| dump | lines | non-target script | latin leak | identical to src | empty | no final punctuation |\n")
	sb.WriteString("|---|--:|--:|--:|--:|--:|--:|\n")
	for _, d := range dumps {
		fmt.Fprintf(&sb, "| `%s` | %d | %d | %d | %d | %d | %d |\n",
			filepath.Base(d.Path), d.Lines, d.NonTarget, d.Leaks, d.Identical, d.Empty, d.NoFinal)
	}
	for _, d := range dumps {
		if len(d.Examples) == 0 {
			continue
		}
		fmt.Fprintf(&sb, "\n%s — %d flagged line(s):\n", filepath.Base(d.Path), len(d.Examples))
		for _, e := range d.Examples {
			leaks := ""
			if len(e.Leaks) > 0 {
				leaks = " leaks=" + strings.Join(e.Leaks, ",")
			}
			fmt.Fprintf(&sb, "  [%s]%s\n    src: %s\n    tr : %s\n", e.Kind, leaks, e.Src, e.Tr)
		}
	}
	return sb.String()
}

// scanRaw reads a raw-reply capture and reports which replies the translate
// parser would reject. It mirrors the production path's tolerance — a leading
// <think> block and surrounding code fences are stripped before the parse —
// so a reply flagged here is one that really costs its batch.
func scanRaw(path string) (rawScan, error) {
	s := rawScan{Path: path}
	f, err := os.Open(path)
	if err != nil {
		return s, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var r rawReply
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			return s, err
		}
		s.Replies++
		reason := parseReason(r.Content)
		if reason == "" {
			continue
		}
		recovered := countParsed(r.Content) > 0
		if recovered {
			s.Lenient++
		} else {
			s.Bad++
		}
		if len(s.Findings) < 20 {
			s.Findings = append(s.Findings, rawFinding{
				N: r.N, Chars: r.Chars, Reason: reason,
				Recovered: recovered, Content: r.Content,
			})
		}
	}
	return s, sc.Err()
}

// parseReason returns the parser's complaint about a reply, or "" if the reply
// is a usable id->translation object.
func parseReason(content string) string {
	var m map[string]string
	if err := json.Unmarshal([]byte(stripReplyWrapper(content)), &m); err != nil {
		return err.Error()
	}
	if len(m) == 0 {
		return "empty object"
	}
	return ""
}

// stripReplyWrapper removes the leading <think> block and the code fences the
// production parser removes before its own json.Unmarshal.
func stripReplyWrapper(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "<think>") {
		_, after, ok := strings.Cut(s, "</think>")
		if !ok {
			return ""
		}
		s = strings.TrimSpace(after)
	}
	if !strings.HasPrefix(s, "```") {
		return s
	}
	s = strings.TrimPrefix(s, "```")
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[i+1:]
	}
	if i := strings.LastIndex(s, "```"); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

func renderRawTable(raws []rawScan) string {
	var sb strings.Builder
	sb.WriteString("\n| raw capture | replies | needed the tolerant parse | unusable |\n")
	sb.WriteString("|---|--:|--:|--:|\n")
	for _, r := range raws {
		fmt.Fprintf(&sb, "| `%s` | %d | %d | %d |\n", filepath.Base(r.Path), r.Replies, r.Lenient, r.Bad)
	}
	for _, r := range raws {
		for _, f := range r.Findings {
			fmt.Fprintf(&sb, "\n%s reply #%d (%d chars) — %s (lenient extraction recovers it: %v)\n%s\n",
				filepath.Base(r.Path), f.N, f.Chars, f.Reason, f.Recovered, f.Content)
		}
	}
	return sb.String()
}

func joinOrDash(v []string) string {
	if len(v) == 0 {
		return "—"
	}
	return strings.Join(v, ", ")
}

func dash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}
