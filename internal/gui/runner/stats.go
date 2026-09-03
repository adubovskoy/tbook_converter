package runner

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
)

// SumStatsCost sums the "cost" field across all lines of a stats JSONL file.
// A missing or unreadable file yields 0; malformed lines are skipped — cost
// accounting must never fail a conversion.
func SumStatsCost(path string) float64 {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()

	var total float64
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var rec struct {
			Cost float64 `json:"cost"`
		}
		if err := json.Unmarshal(line, &rec); err != nil {
			continue
		}
		total += rec.Cost
	}
	return total
}
