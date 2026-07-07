package translate

import "testing"

func TestParseChunks(t *testing.T) {
	cases := []struct {
		name, in string
		ids      int
	}{
		{"plain", `{"a":[{"tgt":"x","en":"X"}],"b":[]}`, 2},
		{"fenced", "```json\n{\"a\":[{\"tgt\":\"x\",\"en\":\"X\"}]}\n```", 1},
		{"prose-wrapped", "Here you go:\n{\"a\":[{\"tgt\":\"x\",\"en\":\"X\"}]}\nDone.", 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m, err := parseChunks(c.in)
			if err != nil {
				t.Fatalf("err: %v", err)
			}
			if len(m) != c.ids {
				t.Fatalf("got %d ids, want %d", len(m), c.ids)
			}
		})
	}
	if _, err := parseChunks("not json at all"); err == nil {
		t.Errorf("expected error on garbage")
	}
}

func TestParseTexts(t *testing.T) {
	cases := []struct {
		name, in string
		ids      int
	}{
		{"plain", `{"a":"Привет","b":"Мир"}`, 2},
		{"fenced", "```json\n{\"a\":\"Привет\"}\n```", 1},
		{"prose-wrapped", "Here:\n{\"a\":\"Привет\"}\nDone.", 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m, err := parseTexts(c.in)
			if err != nil {
				t.Fatalf("err: %v", err)
			}
			if len(m) != c.ids {
				t.Fatalf("got %d ids, want %d", len(m), c.ids)
			}
		})
	}
	if _, err := parseTexts("not json at all"); err == nil {
		t.Errorf("expected error on garbage")
	}
}

func TestBackoffHonorsRetryAfter(t *testing.T) {
	if got := backoff(0, 7_000_000_000); got != 7_000_000_000 {
		t.Errorf("retry-after not honored: %v", got)
	}
}
