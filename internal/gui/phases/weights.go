// Package phases folds the converter's NDJSON progress events into one
// overall percent for the GUI progress bar.
package phases

// baseWeights lists every pipeline phase IN PIPELINE ORDER with its share of
// wall time, as measured by converter_web (jobs_handlers.go): gonka with
// REPAIR_CONTEXT 2 on a 14.7k-sentence book. Order matters: a later phase
// reporting marks the earlier ones for that target complete.
var baseWeights = []phaseWeight{
	{"translate", 0.16},
	{"repair", 0.47},
	{"embalign", 0.26},
	{"align", 0.07},
	{"judge", 0.02},
	{"assemble", 0.02},
}

type phaseWeight struct {
	name   string
	weight float64
}

// Plan is the active phase list with weights renormalized to sum to 1, in
// pipeline order.
type Plan struct {
	phases []phaseWeight
}

// NewPlan drops the phases that will not run and rescales the rest. That is a
// re-scaling, not a re-measurement: the remaining phases keep their measured
// proportions to each other, which is what the bar needs.
func NewPlan(repairOn, judgeOn, embalignOn bool) Plan {
	kept := make([]phaseWeight, 0, len(baseWeights))
	sum := 0.0
	for _, p := range baseWeights {
		switch p.name {
		case "repair":
			if !repairOn {
				continue
			}
		case "judge":
			if !judgeOn {
				continue
			}
		case "embalign":
			if !embalignOn {
				continue
			}
		}
		kept = append(kept, p)
		sum += p.weight
	}
	for i := range kept {
		kept[i].weight /= sum
	}
	return Plan{phases: kept}
}

func (p Plan) index(phase string) int {
	for i, ph := range p.phases {
		if ph.name == phase {
			return i
		}
	}
	return -1
}

type counts struct{ done, total int }

// Tracker folds interleaved per-target progress events into one percent.
// Every per-target phase contributes weight/len(targets); assemble runs once
// globally (its events carry no target) and contributes its full weight.
type Tracker struct {
	plan    Plan
	targets []string
	state   map[string]counts // "phase|target" → latest report
	last    float64           // monotonic clamp
	done    bool
}

func NewTracker(p Plan, targets []string) *Tracker {
	if len(targets) == 0 {
		targets = []string{""}
	}
	return &Tracker{plan: p, targets: targets, state: make(map[string]counts)}
}

// Update records the latest done/total for phase|target. Events for phases
// outside the plan are ignored; "done" marks the whole run finished.
func (t *Tracker) Update(phase, target string, done, total int) {
	if phase == "done" {
		t.done = true
		return
	}
	if t.plan.index(phase) < 0 {
		return
	}
	if phase == "assemble" {
		target = "" // global phase; the event carries no target
	}
	t.state[phase+"|"+target] = counts{done, total}
}

// Percent returns the overall progress in [0,100], non-decreasing: phases can
// end short (done<total) and totals can be revised, so the raw sum is clamped
// against the last returned value.
func (t *Tracker) Percent() float64 {
	pct := t.percent()
	if pct < t.last {
		pct = t.last
	}
	t.last = pct
	return pct
}

func (t *Tracker) percent() float64 {
	if t.done {
		return 100
	}
	n := float64(len(t.targets))
	_, assembleSeen := t.state["assemble|"]
	acc := 0.0
	for _, tgt := range t.targets {
		furthest := t.furthest(tgt)
		for i, ph := range t.plan.phases {
			if ph.name == "assemble" {
				continue
			}
			w := ph.weight / n
			switch {
			case assembleSeen || i < furthest:
				acc += w // a later phase reported: this one is complete even if it ended short
			case i == furthest:
				acc += w * frac(t.state[ph.name+"|"+tgt])
			}
		}
	}
	if c, ok := t.state["assemble|"]; ok {
		if i := t.plan.index("assemble"); i >= 0 {
			acc += t.plan.phases[i].weight * frac(c)
		}
	}
	return acc * 100
}

// furthest is the plan index of the latest per-target phase this target has
// reported, or -1.
func (t *Tracker) furthest(target string) int {
	best := -1
	for i, ph := range t.plan.phases {
		if ph.name == "assemble" {
			continue
		}
		if _, ok := t.state[ph.name+"|"+target]; ok {
			best = i
		}
	}
	return best
}

func frac(c counts) float64 {
	if c.total <= 0 {
		return 0
	}
	f := float64(c.done) / float64(c.total)
	if f > 1 {
		return 1
	}
	return f
}

// Label is the human phrase the GUI shows for a phase.
func Label(phase string) string {
	switch phase {
	case "translate":
		return "Translating sentences"
	case "repair":
		return "Proofreading the translation"
	case "embalign":
		return "Aligning words (fast pass)"
	case "align":
		return "Aligning words"
	case "judge":
		return "Reviewing translation quality"
	case "assemble":
		return "Assembling the book"
	case "done":
		return "Finishing up"
	}
	return ""
}
