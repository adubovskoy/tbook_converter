package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// Pairs mode: replay a blind pairwise set (prepare_pairs.py layout) through Jev
// as one Choice question per presented pair, then score it the way
// analyze_pairs.py scores the LLM judge and measure agreement with that judge.

type pairItem struct {
	ID  string `json:"id"`
	Src string `json:"src"`
	X   string `json:"x"`
	Y   string `json:"y"`
}

// batch files carry uppercase X/Y keys.
func (p *pairItem) UnmarshalJSON(b []byte) error {
	var raw map[string]string
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	p.ID, p.Src, p.X, p.Y = raw["id"], raw["src"], raw["X"], raw["Y"]
	return nil
}

type pairKey struct {
	ALabel string `json:"a_label"`
	BLabel string `json:"b_label"`
	Pairs  map[string]struct {
		Src  string `json:"src"`
		Swap bool   `json:"swap"`
	} `json:"pairs"`
}

type pairOut struct {
	Batch     string             `json:"batch"`
	Order     int                `json:"order"`
	ID        string             `json:"id"`
	Choice    string             `json:"choice"`
	Probs     map[string]float64 `json:"probabilities"`
	Conf      float64            `json:"confidence"`
	LatencyMS int64              `json:"latency_ms"`
	InTok     int                `json:"input_tokens"`
	Cost      float64            `json:"cost"`
	Err       string             `json:"err,omitempty"`
}

var batchRe = regexp.MustCompile(`^batch-(\d+)-o([12])\.json$`)

func pairQuestion(lang string) map[string]question {
	L := langName[lang]
	return map[string]question{
		"better": {Type: "choice",
			Instructions: "Two " + L + " translations, `X` and `Y`, of the English sentence `src`. Which is the better literary translation: more faithful to the meaning, and more natural " + L + "?",
			Criteria: map[string]string{
				"X":   "X is the better translation.",
				"Y":   "Y is the better translation.",
				"tie": "Both are equally good, or equally flawed in different ways.",
			}},
	}
}

func runPairs(dir, lang, out, model, base, key string, conc int) {
	var pk pairKey
	kb, err := os.ReadFile(filepath.Join(dir, "key.json"))
	if err != nil {
		fail(err)
	}
	if err := json.Unmarshal(kb, &pk); err != nil {
		fail(err)
	}
	files, _ := filepath.Glob(filepath.Join(dir, "batch-*-o*.json"))
	sort.Strings(files)

	done := map[string]bool{}
	var prev []pairOut
	if f, err := os.Open(out); err == nil {
		dec := json.NewDecoder(f)
		for {
			var p pairOut
			if err := dec.Decode(&p); err != nil {
				break
			}
			if p.Err == "" {
				done[p.Batch+"/"+p.ID] = true
			}
			prev = append(prev, p)
		}
		f.Close()
	}
	of, err := os.OpenFile(out, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		fail(err)
	}
	enc := json.NewEncoder(of)
	var mu sync.Mutex
	client := &http.Client{Timeout: 60 * time.Second}
	qs := pairQuestion(lang)
	sem := make(chan struct{}, conc)
	var wg sync.WaitGroup
	var nOK, nErr int
	for _, file := range files {
		m := batchRe.FindStringSubmatch(filepath.Base(file))
		if m == nil {
			continue
		}
		order := 1
		if m[2] == "2" {
			order = 2
		}
		bb, err := os.ReadFile(file)
		if err != nil {
			fail(err)
		}
		var items []pairItem
		if err := json.Unmarshal(bb, &items); err != nil {
			fail(fmt.Errorf("%s: %w", file, err))
		}
		bname := strings.TrimSuffix(filepath.Base(file), ".json")
		for _, it := range items {
			if done[bname+"/"+it.ID] {
				continue
			}
			wg.Add(1)
			sem <- struct{}{}
			go func(it pairItem, bname string, order int) {
				defer wg.Done()
				defer func() { <-sem }()
				po := probePair(client, base, key, model, it, qs)
				po.Batch, po.Order = bname, order
				mu.Lock()
				_ = enc.Encode(po)
				if po.Err == "" {
					nOK++
				} else {
					nErr++
					fmt.Fprintf(os.Stderr, "%s/%s: %s\n", bname, it.ID, po.Err)
				}
				prev = append(prev, po)
				mu.Unlock()
			}(it, bname, order)
		}
	}
	wg.Wait()
	of.Close()
	fmt.Fprintf(os.Stderr, "probed %d ok, %d errors, %d already done\n", nOK, nErr, len(done))
	analyzePairs(dir, pk, prev)
}

func probePair(client *http.Client, base, key, model string, it pairItem, qs map[string]question) pairOut {
	po := pairOut{ID: it.ID}
	body, _ := json.Marshal(request{Model: model, State: map[string]string{"src": it.Src, "X": it.X, "Y": it.Y}, Questions: qs})
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
		po.LatencyMS = time.Since(t0).Milliseconds()
		if resp.StatusCode == 429 || resp.StatusCode >= 500 {
			lastErr = strings.TrimSpace(string(raw))
			continue
		}
		var res response
		if err := json.Unmarshal(raw, &res); err != nil || resp.StatusCode != 200 {
			po.Err = fmt.Sprintf("http %d: %s", resp.StatusCode, truncate(string(raw), 300))
			return po
		}
		a, ok := res.Answers["better"]
		if !ok {
			po.Err = "no answer: " + truncate(string(raw), 300)
			return po
		}
		po.Choice = a.Choice
		if a.Confidence != nil {
			po.Conf = *a.Confidence
		}
		_ = json.Unmarshal(a.Probabilities, &po.Probs)
		po.InTok, po.Cost = res.Usage.InputTokens, res.Usage.Cost
		return po
	}
	po.Err = "gave up: " + truncate(lastErr, 300)
	return po
}

// dewrap mirrors analyze_pairs.py: order 1 shows X = (b if swap else a);
// order 2 swaps the presentation again.
func dewrap(pk pairKey, id, v string, order int) string {
	if v != "X" && v != "Y" {
		return "tie"
	}
	swap := pk.Pairs[id].Swap
	pickedFirst := (v == "X")
	if order == 2 {
		pickedFirst = (v == "Y")
	}
	if pickedFirst == swap {
		return pk.BLabel
	}
	return pk.ALabel
}

func loadVerdicts(dir string, order int) map[string]string {
	out := map[string]string{}
	files, _ := filepath.Glob(filepath.Join(dir, fmt.Sprintf("verdict-*-o%d.json", order)))
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		var m map[string]string
		if json.Unmarshal(b, &m) == nil {
			for k, v := range m {
				out[k] = v
			}
		}
	}
	return out
}

func binomTwoSided(k, n int) float64 {
	if n == 0 {
		return 1
	}
	lo := k
	if n-k < lo {
		lo = n - k
	}
	var p float64
	for i := 0; i <= lo; i++ {
		p += math.Exp(lgammaChoose(n, i))
	}
	p = p / math.Pow(2, float64(n)) * 2
	return math.Min(1, p)
}

func lgammaChoose(n, k int) float64 {
	a, _ := math.Lgamma(float64(n + 1))
	b, _ := math.Lgamma(float64(k + 1))
	c, _ := math.Lgamma(float64(n - k + 1))
	return a - b - c
}

func analyzePairs(dir string, pk pairKey, recs []pairOut) {
	// Jev's own verdicts per order, with the winning probability.
	type pick struct {
		v    string
		pmax float64
	}
	jev := map[int]map[string]pick{1: {}, 2: {}}
	var lat []float64
	var cost float64
	var inTok int
	for _, r := range recs {
		if r.Err != "" {
			continue
		}
		pm := 0.0
		for _, p := range r.Probs {
			pm = math.Max(pm, p)
		}
		jev[r.Order][r.ID] = pick{r.Choice, pm}
		lat = append(lat, float64(r.LatencyMS))
		cost += r.Cost
		inTok += r.InTok
	}
	if len(lat) == 0 {
		fmt.Println("no successful pair records")
		return
	}
	sort.Float64s(lat)
	fmt.Printf("pairs dir %s: %d presentations judged, latency p50 %.0f ms p90 %.0f ms, input tokens %d, cost $%.4f\n",
		filepath.Base(dir), len(lat), lat[len(lat)/2], lat[int(0.9*float64(len(lat)-1))], inTok, cost)

	llm1, llm2 := loadVerdicts(dir, 1), loadVerdicts(dir, 2)
	ids := make([]string, 0, len(pk.Pairs))
	for id := range pk.Pairs {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	// Sweep a minimum winning-probability for Jev to count as decisive.
	for _, tmin := range []float64{0, 0.5, 0.6, 0.7, 0.8} {
		wins := map[string]int{pk.ALabel: 0, pk.BLabel: 0}
		var ties, inconsistent, missing int
		// agreement with the LLM judge on pairs where both are decisive
		var both, agree int
		// on LLM-decisive pairs: what Jev says
		var llmDec, jevAgree, jevOpp, jevTie int
		for _, id := range ids {
			j1, ok1 := jev[1][id]
			j2, ok2 := jev[2][id]
			if !ok1 || !ok2 {
				missing++
				continue
			}
			v1, v2 := j1.v, j2.v
			if j1.pmax < tmin {
				v1 = "tie"
			}
			if j2.pmax < tmin {
				v2 = "tie"
			}
			r1, r2 := dewrap(pk, id, v1, 1), dewrap(pk, id, v2, 2)
			jres := "tie"
			switch {
			case r1 == "tie" || r2 == "tie":
				ties++
			case r1 != r2:
				inconsistent++
			default:
				wins[r1]++
				jres = r1
			}
			l1, l2 := llm1[id], llm2[id]
			if l1 == "" || l2 == "" {
				continue
			}
			lr1, lr2 := dewrap(pk, id, l1, 1), dewrap(pk, id, l2, 2)
			lres := "tie"
			if lr1 != "tie" && lr2 != "tie" && lr1 == lr2 {
				lres = lr1
			}
			if lres != "tie" {
				llmDec++
				switch {
				case jres == lres:
					jevAgree++
				case jres == "tie":
					jevTie++
				default:
					jevOpp++
				}
			}
			if lres != "tie" && jres != "tie" {
				both++
				if lres == jres {
					agree++
				}
			}
		}
		dec := wins[pk.ALabel] + wins[pk.BLabel]
		fmt.Printf("\n[jev decisive when winning p >= %.1f]\n", tmin)
		fmt.Printf("pairs: %d  judged: %d  missing: %d\n", len(ids), len(ids)-missing, missing)
		fmt.Printf("ties: %d  order-inconsistent (counted as ties): %d\n", ties, inconsistent)
		fmt.Printf("decisive: %d\n", dec)
		for _, l := range []string{pk.ALabel, pk.BLabel} {
			share := 0.0
			if dec > 0 {
				share = float64(wins[l]) / float64(dec)
			}
			fmt.Printf("  %s: %d  (%.1f%% of decisive)\n", l, wins[l], 100*share)
		}
		if dec > 0 {
			lo := wins[pk.ALabel]
			if wins[pk.BLabel] < lo {
				lo = wins[pk.BLabel]
			}
			fmt.Printf("sign test two-sided p = %.4g\n", binomTwoSided(lo, dec))
		}
		if both > 0 {
			fmt.Printf("agreement with the LLM judge where both decisive: %d/%d = %.1f%%\n", agree, both, 100*float64(agree)/float64(both))
		}
		if llmDec > 0 {
			fmt.Printf("on the %d LLM-decisive pairs: jev agrees %d (%.1f%%), opposes %d (%.1f%%), tie/inconsistent %d (%.1f%%)\n",
				llmDec, jevAgree, 100*float64(jevAgree)/float64(llmDec), jevOpp, 100*float64(jevOpp)/float64(llmDec), jevTie, 100*float64(jevTie)/float64(llmDec))
		}
	}
}
