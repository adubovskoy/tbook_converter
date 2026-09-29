package v2

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// A small evaluator for the JSON Schema draft 2020-12 subset the shipped schema
// uses. It validates the shape of an entry against one of the schema's $defs;
// cross-entry rules stay in validate.go. Keywords: $ref (#/$defs/… only), type,
// const, enum, required, properties, additionalProperties, items, prefixItems,
// minItems, maxItems, uniqueItems, minProperties, maxProperties, minimum,
// minLength, pattern, oneOf, anyOf. Others are documentation.

type schemaDoc struct {
	defs     map[string]any
	patterns map[string]*regexp.Regexp
}

var (
	schemaOnce sync.Once
	schemaCur  *schemaDoc
	schemaErr  error
)

func shippedSchema() (*schemaDoc, error) {
	schemaOnce.Do(func() {
		var root map[string]any
		if err := json.Unmarshal(SchemaJSON, &root); err != nil {
			schemaErr = err
			return
		}
		defs, _ := root["$defs"].(map[string]any)
		schemaCur = &schemaDoc{defs: defs, patterns: map[string]*regexp.Regexp{}}
	})
	return schemaCur, schemaErr
}

// decodeShape parses JSON keeping numbers as json.Number so integer checks
// are exact.
func decodeShape(data []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	if dec.More() {
		return nil, fmt.Errorf("trailing data after the JSON value")
	}
	return v, nil
}

// ShapeErrors validates v (as returned by decodeShape) against $defs/<def>
// of the shipped schema and returns one message per violation.
func ShapeErrors(def string, v any) []string {
	d, err := shippedSchema()
	if err != nil {
		return []string{"schema: " + err.Error()}
	}
	s, ok := d.defs[def]
	if !ok {
		return []string{"schema: no definition " + def}
	}
	var errs []string
	d.check(s, v, "$", &errs)
	return errs
}

func (d *schemaDoc) resolve(s any) (map[string]any, bool) {
	switch x := s.(type) {
	case bool:
		return nil, x // true = anything, false = nothing
	case map[string]any:
		if ref, ok := x["$ref"].(string); ok {
			name := strings.TrimPrefix(ref, "#/$defs/")
			if target, ok := d.defs[name]; ok {
				return d.resolve(target)
			}
			return nil, true
		}
		return x, true
	}
	return nil, true
}

func (d *schemaDoc) check(schema any, v any, path string, errs *[]string) {
	s, allow := d.resolve(schema)
	if s == nil {
		if !allow {
			*errs = append(*errs, path+": not allowed")
		}
		return
	}
	fail := func(f string, a ...any) { *errs = append(*errs, path+": "+fmt.Sprintf(f, a...)) }

	if t, ok := s["type"]; ok && !typeMatches(t, v) {
		fail("want type %v, got %s", t, jsonType(v))
		return
	}
	if c, ok := s["const"]; ok && !jsonEqual(c, v) {
		fail("must equal %v", c)
	}
	if e, ok := s["enum"].([]any); ok {
		hit := false
		for _, c := range e {
			if jsonEqual(c, v) {
				hit = true
				break
			}
		}
		if !hit {
			fail("must be one of %v", e)
		}
	}
	if alts, ok := s["oneOf"].([]any); ok {
		n, nearest := d.alternatives(alts, v, path)
		if n != 1 {
			fail("matches %d of %d alternatives, want exactly 1", n, len(alts))
			if n == 0 {
				*errs = append(*errs, nearest...)
			}
		}
	}
	if alts, ok := s["anyOf"].([]any); ok {
		if n, nearest := d.alternatives(alts, v, path); n == 0 {
			fail("matches no alternative")
			*errs = append(*errs, nearest...)
		}
	}

	switch x := v.(type) {
	case map[string]any:
		if req, ok := s["required"].([]any); ok {
			for _, r := range req {
				if _, has := x[r.(string)]; !has {
					fail("missing required %q", r)
				}
			}
		}
		if n, ok := number(s["minProperties"]); ok && float64(len(x)) < n {
			fail("want at least %v properties", n)
		}
		if n, ok := number(s["maxProperties"]); ok && float64(len(x)) > n {
			fail("want at most %v properties", n)
		}
		props, _ := s["properties"].(map[string]any)
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if ps, ok := props[k]; ok {
				d.check(ps, x[k], path+"."+k, errs)
			} else if ap, ok := s["additionalProperties"]; ok {
				d.check(ap, x[k], path+"."+k, errs)
			}
		}
	case []any:
		if n, ok := number(s["minItems"]); ok && float64(len(x)) < n {
			fail("want at least %v items", n)
		}
		if n, ok := number(s["maxItems"]); ok && float64(len(x)) > n {
			fail("want at most %v items", n)
		}
		prefix, _ := s["prefixItems"].([]any)
		for i, item := range x {
			p := fmt.Sprintf("%s[%d]", path, i)
			if i < len(prefix) {
				d.check(prefix[i], item, p, errs)
			} else if items, ok := s["items"]; ok {
				d.check(items, item, p, errs)
			}
		}
		if u, _ := s["uniqueItems"].(bool); u {
			for i := range x {
				for j := i + 1; j < len(x); j++ {
					if jsonEqual(x[i], x[j]) {
						fail("items %d and %d are equal", i, j)
					}
				}
			}
		}
	case string:
		if n, ok := number(s["minLength"]); ok && float64(len([]rune(x))) < n {
			fail("shorter than %v", n)
		}
		if p, ok := s["pattern"].(string); ok {
			re := d.patterns[p]
			if re == nil {
				re = regexp.MustCompile(p)
				d.patterns[p] = re
			}
			if !re.MatchString(x) {
				fail("%q does not match %s", x, p)
			}
		}
	case json.Number:
		if lo, ok := number(s["minimum"]); ok {
			if f, _ := x.Float64(); f < lo {
				fail("%v below minimum %v", x, lo)
			}
		}
	}
}

// alternatives counts the matching alternatives and returns the errors of the
// nearest failing one: the alternative whose required keys are present, then
// the one with the fewest errors.
func (d *schemaDoc) alternatives(alts []any, v any, path string) (int, []string) {
	n, best, bestScore := 0, []string(nil), 0
	for _, alt := range alts {
		var sub []string
		d.check(alt, v, path, &sub)
		if len(sub) == 0 {
			n++
			continue
		}
		// Distance: a type mismatch outweighs everything, then missing
		// required keys, then the error count.
		score := len(sub)
		if as, ok := d.resolve(alt); ok && as != nil {
			if t, ok := as["type"]; ok && !typeMatches(t, v) {
				score += 1 << 20
			}
			if obj, ok := v.(map[string]any); ok {
				if req, ok := as["required"].([]any); ok {
					for _, r := range req {
						if _, has := obj[r.(string)]; !has {
							score += 1 << 10
						}
					}
				}
			}
		}
		if best == nil || score < bestScore {
			best, bestScore = sub, score
		}
	}
	return n, best
}

func typeMatches(t any, v any) bool {
	switch tt := t.(type) {
	case string:
		return typeIs(tt, v)
	case []any:
		for _, one := range tt {
			if s, ok := one.(string); ok && typeIs(s, v) {
				return true
			}
		}
	}
	return false
}

func typeIs(t string, v any) bool {
	switch t {
	case "null":
		return v == nil
	case "boolean":
		_, ok := v.(bool)
		return ok
	case "string":
		_, ok := v.(string)
		return ok
	case "array":
		_, ok := v.([]any)
		return ok
	case "object":
		_, ok := v.(map[string]any)
		return ok
	case "number":
		_, ok := v.(json.Number)
		return ok
	case "integer":
		n, ok := v.(json.Number)
		if !ok {
			return false
		}
		if _, err := n.Int64(); err == nil {
			return true
		}
		f, err := n.Float64()
		return err == nil && f == math.Trunc(f)
	}
	return false
}

func jsonType(v any) string {
	switch v.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case string:
		return "string"
	case json.Number:
		return "number"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	}
	return reflect.TypeOf(v).String()
}

// jsonEqual compares a schema constant (decoded with float64 numbers) with a
// value decoded with json.Number.
func jsonEqual(c any, v any) bool {
	cn, cok := number(c)
	vn, vok := number(v)
	if cok || vok {
		return cok && vok && cn == vn
	}
	return reflect.DeepEqual(c, v)
}

func number(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case json.Number:
		f, err := x.Float64()
		return f, err == nil
	}
	return 0, false
}
