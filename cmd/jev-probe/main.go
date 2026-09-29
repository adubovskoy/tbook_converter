// Command jev-probe measures TypeSafe's Jev (a System One decision model) as a
// translation-quality gate, through OpenRouter's /systemone endpoint.
//
// Input is a GOLD-lite JSONL (bench-quality/gold/en-<lang>.jsonl): every record
// carries the English source, a reference translation and the checker's label
// (ok | minor | wrong). For each record Jev is asked a fixed set of questions
// about the pair; the answers, latency and usage go to an output JSONL, and a
// summary prints how well the probabilities separate the checker's classes.
//
// The OpenRouter key is read through the same .env loader the converter uses
// and is never printed.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/joho/godotenv"
)

type goldRec struct {
	ID          string `json:"id"`
	Src         string `json:"src"`
	Ref         string `json:"ref"`
	Status      string `json:"status"`
	Check       string `json:"check"`
	CheckReason string `json:"check_reason"`
}

type question struct {
	Type         string      `json:"type"`
	Instructions string      `json:"instructions"`
	Criteria     interface{} `json:"criteria,omitempty"`
}

type request struct {
	Model     string              `json:"model"`
	State     map[string]string   `json:"state"`
	Questions map[string]question `json:"questions"`
}

type answer struct {
	Type          string             `json:"type"`
	Noul          *float64           `json:"noul,omitempty"`
	Choice        string             `json:"choice,omitempty"`
	Score         *float64           `json:"score,omitempty"`
	Confidence    *float64           `json:"confidence,omitempty"`
	Probabilities json.RawMessage    `json:"probabilities,omitempty"`
	Legend        json.RawMessage    `json:"legend,omitempty"`
	Extra         map[string]any     `json:"-"`
}

type response struct {
	ID       string            `json:"id"`
	Model    string            `json:"model"`
	Provider string            `json:"provider"`
	Answers  map[string]answer `json:"answers"`
	Usage    struct {
		InputTokens  int     `json:"input_tokens"`
		OutputTokens int     `json:"output_tokens"`
		Cost         float64 `json:"cost"`
	} `json:"usage"`
	Error json.RawMessage `json:"error,omitempty"`
}

type outRec struct {
	ID        string            `json:"id"`
	Check     string            `json:"check"`
	Status    string            `json:"status"`
	Src       string            `json:"src"`
	Ref       string            `json:"ref"`
	Answers   map[string]answer `json:"answers,omitempty"`
	Model     string            `json:"model,omitempty"`
	LatencyMS int64             `json:"latency_ms"`
	InTok     int               `json:"input_tokens"`
	OutTok    int               `json:"output_tokens"`
	Cost      float64           `json:"cost"`
	HTTP      int               `json:"http"`
	Err       string            `json:"err,omitempty"`
}

var langName = map[string]string{"ru": "Russian", "de": "German", "fr": "French"}

func questions(lang string) map[string]question {
	L := langName[lang]
	return map[string]question{
		"faithful": {Type: "noul",
			Instructions: "The translation `tr` conveys exactly the meaning of the English sentence `src`: nothing is added, nothing is omitted, and no word is mistranslated.",
			Criteria: map[string]string{
				"true":  "Every idea in `src` appears in `tr` with the same sense and nothing extra appears.",
				"false": "`tr` invents, drops or changes meaning, or mistranslates a word."}},
		"natural": {Type: "noul",
			Instructions: "`tr` reads as natural, grammatical " + L + " that a native speaker would write.",
			Criteria: map[string]string{
				"true":  "Fluent, idiomatic " + L + ".",
				"false": "Calques, agreement errors, unnatural word order or non-words."}},
		"target_lang": {Type: "noul",
			Instructions: "`tr` is written in " + L + ".",
			Criteria: map[string]string{
				"true":  "The text of `tr` is " + L + ".",
				"false": "`tr` is English or any other language."}},
		"verdict": {Type: "score",
			Instructions: "Rate the translation `tr` of the English sentence `src` as a professional literary translation reviewer would.",
			Criteria: []string{
				"wrong: the translation changes, invents or drops meaning, or mistranslates a word",
				"minor: the meaning is kept but a word choice, nuance, register or wording is off",
				"ok: an accurate, natural translation with nothing to fix",
			}},
	}
}

func main() {
	var (
		gold    = flag.String("gold", "", "GOLD-lite JSONL (bench-quality/gold/en-<lang>.jsonl)")
		lang    = flag.String("lang", "ru", "target language code: ru | de | fr")
		out     = flag.String("out", "", "output JSONL (answers per record)")
		model   = flag.String("model", "jev-latest", "System One model id")
		limit   = flag.Int("limit", 0, "probe only the first N records (0 = all)")
		onlyIDs = flag.String("ids", "", "comma-separated record ids to probe (overrides --limit)")
		conc    = flag.Int("concurrency", 8, "parallel requests")
		base    = flag.String("base", "https://openrouter.ai/api/v1", "API base URL")
		summ    = flag.String("summarize", "", "skip the network: summarize an existing output JSONL")
		pairs   = flag.String("pairs", "", "pairs mode: a prepare_pairs.py directory (batch-*-o*.json, key.json, verdict-*.json)")
	)
	flag.Parse()

	if *pairs != "" {
		if *out == "" {
			fail(fmt.Errorf("--out is required"))
		}
		_ = godotenv.Load()
		key := os.Getenv("OPENROUTER_API_KEY")
		if key == "" {
			fail(fmt.Errorf("OPENROUTER_API_KEY is not set (put it in .env; it is never printed)"))
		}
		runPairs(*pairs, *lang, *out, *model, *base, key, *conc)
		return
	}
	if *summ != "" {
		recs, err := readOut(*summ)
		if err != nil {
			fail(err)
		}
		summarize(recs)
		return
	}
	if *gold == "" || *out == "" {
		fail(fmt.Errorf("--gold and --out are required"))
	}
	_ = godotenv.Load()
	key := os.Getenv("OPENROUTER_API_KEY")
	if key == "" {
		fail(fmt.Errorf("OPENROUTER_API_KEY is not set (put it in .env; it is never printed)"))
	}

	f, err := os.Open(*gold)
	if err != nil {
		fail(err)
	}
	var recs []goldRec
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	want := map[string]bool{}
	for _, id := range strings.Split(*onlyIDs, ",") {
		if id = strings.TrimSpace(id); id != "" {
			want[id] = true
		}
	}
	for sc.Scan() {
		var r goldRec
		if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
			fail(err)
		}
		if len(want) > 0 && !want[r.ID] {
			continue
		}
		recs = append(recs, r)
	}
	f.Close()
	if *limit > 0 && len(want) == 0 && len(recs) > *limit {
		recs = recs[:*limit]
	}

	done := map[string]bool{}
	if prev, err := readOut(*out); err == nil {
		for _, p := range prev {
			if p.Err == "" {
				done[p.ID] = true
			}
		}
	}
	of, err := os.OpenFile(*out, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		fail(err)
	}
	defer of.Close()
	var mu sync.Mutex
	enc := json.NewEncoder(of)

	qs := questions(*lang)
	client := &http.Client{Timeout: 60 * time.Second}
	sem := make(chan struct{}, *conc)
	var wg sync.WaitGroup
	var nOK, nErr int
	for _, r := range recs {
		if done[r.ID] {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(r goldRec) {
			defer wg.Done()
			defer func() { <-sem }()
			o := probe(client, *base, key, *model, r, qs)
			mu.Lock()
			_ = enc.Encode(o)
			if o.Err == "" {
				nOK++
			} else {
				nErr++
				fmt.Fprintf(os.Stderr, "%s: http %d %s\n", o.ID, o.HTTP, o.Err)
			}
			mu.Unlock()
		}(r)
	}
	wg.Wait()
	fmt.Fprintf(os.Stderr, "probed %d ok, %d errors, %d already done\n", nOK, nErr, len(done))

	all, err := readOut(*out)
	if err != nil {
		fail(err)
	}
	summarize(all)
}

func probe(client *http.Client, base, key, model string, r goldRec, qs map[string]question) outRec {
	o := outRec{ID: r.ID, Check: r.Check, Status: r.Status, Src: r.Src, Ref: r.Ref}
	body, _ := json.Marshal(request{Model: model, State: map[string]string{"src": r.Src, "tr": r.Ref}, Questions: qs})
	var lastErr string
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt*attempt) * 2 * time.Second)
		}
		req, _ := http.NewRequest("POST", strings.TrimRight(base, "/")+"/systemone", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+key)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("HTTP-Referer", "https://tbook.dev")
		req.Header.Set("X-Title", "tbook jev-probe")
		t0 := time.Now()
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err.Error()
			continue
		}
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()
		o.LatencyMS = time.Since(t0).Milliseconds()
		o.HTTP = resp.StatusCode
		if resp.StatusCode == 429 || resp.StatusCode >= 500 {
			lastErr = strings.TrimSpace(string(raw))
			continue
		}
		var res response
		if err := json.Unmarshal(raw, &res); err != nil {
			o.Err = "bad json: " + truncate(string(raw), 300)
			return o
		}
		if resp.StatusCode != 200 || len(res.Answers) == 0 {
			o.Err = truncate(string(raw), 400)
			return o
		}
		o.Answers, o.Model = res.Answers, res.Model
		o.InTok, o.OutTok, o.Cost = res.Usage.InputTokens, res.Usage.OutputTokens, res.Usage.Cost
		return o
	}
	o.Err = "gave up: " + truncate(lastErr, 300)
	return o
}

func readOut(path string) ([]outRec, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var recs []outRec
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	for sc.Scan() {
		var o outRec
		if err := json.Unmarshal(sc.Bytes(), &o); err != nil {
			return nil, err
		}
		recs = append(recs, o)
	}
	return recs, nil
}

// summarize prints, per checker class, the mean of each noul answer and the
// score verdict, then the separation (AUROC) of "faithful" and of the score
// between ok and the two defect classes, plus latency and cost.
func summarize(recs []outRec) {
	var ok []outRec
	var lat []float64
	var cost float64
	var inTok int
	for _, r := range recs {
		if r.Err != "" {
			continue
		}
		ok = append(ok, r)
		lat = append(lat, float64(r.LatencyMS))
		cost += r.Cost
		inTok += r.InTok
	}
	if len(ok) == 0 {
		fmt.Println("no successful records")
		return
	}
	sort.Float64s(lat)
	pct := func(p float64) float64 { return lat[int(math.Min(float64(len(lat)-1), math.Floor(p*float64(len(lat)))))] }
	fmt.Printf("records %d  latency p50 %.0f ms  p90 %.0f ms  max %.0f ms  input tokens %d  cost $%.5f  model %s\n",
		len(ok), pct(0.5), pct(0.9), lat[len(lat)-1], inTok, cost, ok[0].Model)

	classes := []string{"ok", "minor", "wrong"}
	keys := []string{"faithful", "natural", "target_lang"}
	fmt.Printf("\n%-6s %5s", "class", "n")
	for _, k := range keys {
		fmt.Printf(" %12s", "mean "+k)
	}
	fmt.Printf(" %12s\n", "mean score")
	for _, c := range classes {
		var n int
		sum := map[string]float64{}
		var scoreSum float64
		var scoreN int
		for _, r := range ok {
			if r.Check != c {
				continue
			}
			n++
			for _, k := range keys {
				if a, has := r.Answers[k]; has && a.Noul != nil {
					sum[k] += *a.Noul
				}
			}
			if a, has := r.Answers["verdict"]; has && a.Score != nil {
				scoreSum += *a.Score
				scoreN++
			}
		}
		if n == 0 {
			continue
		}
		fmt.Printf("%-6s %5d", c, n)
		for _, k := range keys {
			fmt.Printf(" %12.3f", sum[k]/float64(n))
		}
		if scoreN > 0 {
			fmt.Printf(" %12.3f", scoreSum/float64(scoreN))
		}
		fmt.Println()
	}

	sig := func(r outRec, k string) (float64, bool) {
		a, has := r.Answers[k]
		if !has {
			return 0, false
		}
		if a.Noul != nil {
			return *a.Noul, true
		}
		if a.Score != nil {
			return *a.Score, true
		}
		return 0, false
	}
	fmt.Println()
	for _, k := range []string{"faithful", "natural", "verdict"} {
		for _, neg := range []string{"wrong", "minor"} {
			var pos, negs []float64
			for _, r := range ok {
				v, has := sig(r, k)
				if !has {
					continue
				}
				switch r.Check {
				case "ok":
					pos = append(pos, v)
				case neg:
					negs = append(negs, v)
				}
			}
			if len(pos) > 0 && len(negs) > 0 {
				fmt.Printf("AUROC %-9s ok(%d) vs %-5s(%d): %.3f\n", k, len(pos), neg, len(negs), auroc(pos, negs))
			}
		}
	}

	// Operating points for "faithful" as a gate: flag when faithful < t.
	fmt.Println()
	for _, t := range []float64{0.5, 0.7, 0.8, 0.9, 0.95} {
		var flagged, caughtWrong, caughtMinor, fp, nWrong, nMinor, nOK int
		for _, r := range ok {
			v, has := sig(r, "faithful")
			if !has {
				continue
			}
			switch r.Check {
			case "wrong":
				nWrong++
			case "minor":
				nMinor++
			default:
				nOK++
			}
			if v < t {
				flagged++
				switch r.Check {
				case "wrong":
					caughtWrong++
				case "minor":
					caughtMinor++
				default:
					fp++
				}
			}
		}
		fmt.Printf("gate faithful<%.2f: flags %3d/%d (%.1f%%)  wrong caught %d/%d  minor caught %d/%d  false flags on ok %d/%d (%.1f%%)\n",
			t, flagged, len(ok), 100*float64(flagged)/float64(len(ok)), caughtWrong, nWrong, caughtMinor, nMinor, fp, nOK, 100*float64(fp)/float64(max(nOK, 1)))
	}

	// The lowest "faithful" scores, so the reader sees what Jev flags.
	sort.Slice(ok, func(i, j int) bool {
		vi, _ := sig(ok[i], "faithful")
		vj, _ := sig(ok[j], "faithful")
		return vi < vj
	})
	fmt.Println("\nlowest faithful:")
	for i := 0; i < len(ok) && i < 12; i++ {
		v, _ := sig(ok[i], "faithful")
		s, _ := sig(ok[i], "verdict")
		fmt.Printf("  %.3f score %.2f [%s] %s | %s || %s\n", v, s, ok[i].Check, truncate(ok[i].Src, 70), truncate(ok[i].Ref, 70), ok[i].ID)
	}
}

// auroc is the probability that a random ok record scores higher than a
// random defect record (ties count half).
func auroc(pos, neg []float64) float64 {
	var wins float64
	for _, p := range pos {
		for _, n := range neg {
			switch {
			case p > n:
				wins++
			case p == n:
				wins += 0.5
			}
		}
	}
	return wins / float64(len(pos)*len(neg))
}

func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "jev-probe:", err)
	os.Exit(1)
}
