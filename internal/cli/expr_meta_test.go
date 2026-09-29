package cli

import (
	"testing"
	"time"

	"github.com/dimando/reader/converter/internal/cache"
	"github.com/dimando/reader/converter/internal/translate"
)

func TestExpressionsNamespaceAndMeta(t *testing.T) {
	cfg := fullConfig()
	if nsModel(cfg) != cfg.Model {
		t.Fatalf("default namespace = %q, want the plain model", nsModel(cfg))
	}
	r := metaRun(cfg, cfg.Targets, 0, time.Now())
	if r.Prompts["translate"] != cache.TrPromptVersion || r.Prompts["expr"] != "" || r.Options["expressions"] != nil {
		t.Errorf("default run records an expression variant: %+v %+v", r.Prompts, r.Options)
	}

	cfg.Expressions, cfg.NoMirror = true, true
	if nsModel(cfg) == cfg.Model {
		t.Error("expression run shares the production cache namespace")
	}
	r = metaRun(cfg, cfg.Targets, 0, time.Now())
	if r.Prompts["translate"] != cache.TrPromptVersion+translate.TranslateVariant(true, true) ||
		r.Prompts["expr"] != translate.ExprPromptVersion || r.Models["expr"] != cfg.Model ||
		r.Options["expressions"] != true || r.Options["noMirror"] != true {
		t.Errorf("expression run provenance: %+v %+v %+v", r.Prompts, r.Models, r.Options)
	}
}
