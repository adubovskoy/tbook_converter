// Package bootstrap provisions the converter's optional local dependencies
// for the GUI. Lexicons feed the free lexcheck drift gate; a missing lexicon
// is a converter notice, not an error, so every fetch here is best-effort.
package bootstrap

import (
	"bufio"
	"compress/gzip"
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// lexiconBaseURL serves OPUS OpenSubtitles v2018 .dic dumps (same source as
// tools/fetch-lexicons.sh).
const lexiconBaseURL = "https://object.pouta.csc.fi/OPUS-OpenSubtitles/v2018/dic"

// coveredLangs is the OPUS OpenSubtitles coverage the converter's lexicons
// use; other pairs are skipped silently.
var coveredLangs = map[string]bool{"de": true, "en": true, "es": true, "fr": true, "it": true, "ru": true}

// letters mirrors the shell script's Python filter [^\W\d_]: letters and
// combining marks only.
var letters = regexp.MustCompile(`^[\p{L}\p{M}]+$`)

// LexiconCovered reports whether the pair has an OPUS dictionary at all.
func LexiconCovered(src, tgt string) bool {
	return src != tgt && coveredLangs[src] && coveredLangs[tgt]
}

// LexiconPresent reports whether dir already holds the src→tgt lexicon.
func LexiconPresent(dir, src, tgt string) bool {
	_, err := os.Stat(filepath.Join(dir, src+"-"+tgt+".tsv.gz"))
	return err == nil
}

// FetchLexicons downloads the OPUS dictionary for the pair and writes both
// directions (<src>-<tgt>.tsv.gz and <tgt>-<src>.tsv.gz) into dir, exactly
// like tools/fetch-lexicons.sh. No-op when the pair is uncovered or both
// files exist. baseURL "" means the public OPUS object store.
func FetchLexicons(ctx context.Context, dir, src, tgt, baseURL string) error {
	if !LexiconCovered(src, tgt) {
		return nil
	}
	if LexiconPresent(dir, src, tgt) && LexiconPresent(dir, tgt, src) {
		return nil
	}
	if baseURL == "" {
		baseURL = lexiconBaseURL
	}
	// OPUS names pairs alphabetically: en-ru.dic.gz holds en in column 2.
	a, b := src, tgt
	if a > b {
		a, b = b, a
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/%s-%s.dic.gz", baseURL, a, b), nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("opus %s-%s: HTTP %d", a, b, resp.StatusCode)
	}
	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return err
	}
	defer gz.Close()

	type entry struct {
		count int
		word  string
	}
	fwd := map[string][]entry{} // a → b
	rev := map[string][]entry{} // b → a
	sc := bufio.NewScanner(gz)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		parts := strings.Split(sc.Text(), "\t")
		if len(parts) < 6 {
			continue
		}
		count, err := strconv.Atoi(parts[0])
		if err != nil {
			continue
		}
		s := strings.ToLower(strings.TrimSpace(parts[2]))
		t := strings.ToLower(strings.TrimSpace(parts[3]))
		if count < 3 || !letters.MatchString(s) || !letters.MatchString(t) {
			continue
		}
		fwd[s] = append(fwd[s], entry{count, t})
		rev[t] = append(rev[t], entry{count, s})
	}
	if err := sc.Err(); err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	dump := func(d map[string][]entry, path string) error {
		heads := make([]string, 0, len(d))
		for s := range d {
			heads = append(heads, s)
		}
		sort.Strings(heads)
		f, err := os.Create(path)
		if err != nil {
			return err
		}
		w := gzip.NewWriter(f)
		bw := bufio.NewWriter(w)
		for _, s := range heads {
			es := d[s]
			// Python sorts (count, word) tuples descending: count desc,
			// then word desc — replicated so outputs stay byte-comparable.
			sort.Slice(es, func(i, j int) bool {
				if es[i].count != es[j].count {
					return es[i].count > es[j].count
				}
				return es[i].word > es[j].word
			})
			if len(es) > 12 {
				es = es[:12] // cut before dedupe, like the script
			}
			seen := map[string]bool{}
			tops := make([]string, 0, len(es))
			for _, e := range es {
				if !seen[e.word] {
					seen[e.word] = true
					tops = append(tops, e.word)
				}
			}
			fmt.Fprintf(bw, "%s\t%s\n", s, strings.Join(tops, "|"))
		}
		if err := bw.Flush(); err != nil {
			f.Close()
			return err
		}
		if err := w.Close(); err != nil {
			f.Close()
			return err
		}
		return f.Close()
	}
	if err := dump(fwd, filepath.Join(dir, a+"-"+b+".tsv.gz")); err != nil {
		return err
	}
	return dump(rev, filepath.Join(dir, b+"-"+a+".tsv.gz"))
}
