package stopwords

import (
	"encoding/json"
	"os"
	"testing"
)

// exportPath is the checked-in copy tools/embalign.py reads for the
// closed-class guard; cmd/stopwordsdump regenerates it.
const exportPath = "../../tools/stopwords.json"

func TestExportedFileMatches(t *testing.T) {
	b, err := os.ReadFile(exportPath)
	if err != nil {
		t.Fatalf("read %s: %v — regenerate with: go run ./cmd/stopwordsdump -o tools/stopwords.json", exportPath, err)
	}
	var got map[string][]string
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("parse %s: %v", exportPath, err)
	}
	langs := Langs()
	if len(got) != len(langs) {
		t.Errorf("%s has %d languages, Go has %d", exportPath, len(got), len(langs))
	}
	for _, lang := range langs {
		words, ok := got[lang]
		if !ok {
			t.Errorf("%s is missing language %q", exportPath, lang)
			continue
		}
		set := Set(lang)
		if len(words) != len(set) {
			t.Errorf("%s[%q] has %d words, Go has %d", exportPath, lang, len(words), len(set))
		}
		for _, w := range words {
			if !set[w] {
				t.Errorf("%s[%q] has %q, Go does not", exportPath, lang, w)
			}
		}
		for w := range set {
			found := false
			for _, x := range words {
				if x == w {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Go %q has %q, %s does not", lang, w, exportPath)
			}
		}
	}
	for lang := range got {
		if Set(lang) == nil {
			t.Errorf("%s has language %q, Go does not", exportPath, lang)
		}
	}
}
