package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/dimando/reader/converter/internal/buildinfo"
	"github.com/dimando/reader/converter/internal/gui/bootstrap"
	"github.com/dimando/reader/converter/internal/gui/jobs"
	"github.com/dimando/reader/converter/internal/gui/keycheck"
	"github.com/dimando/reader/converter/internal/gui/paths"
	"github.com/dimando/reader/converter/internal/gui/phases"
	"github.com/dimando/reader/converter/internal/gui/pricing"
	convrunner "github.com/dimando/reader/converter/internal/gui/runner"
	"github.com/dimando/reader/converter/internal/gui/settings"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// ConvertService is the whole Go surface bound to the frontend. All Wails
// API usage stays in this package; the internal/gui packages are headless.
type ConvertService struct {
	dirs    paths.Dirs
	store   *settings.Store
	runner  *convrunner.Runner
	manager *jobs.Manager
	prices  *pricing.Fetcher

	mu        sync.Mutex
	trackerID string
	tracker   *phases.Tracker
}

func NewConvertService() (*ConvertService, error) {
	dirs, err := paths.Resolve()
	if err != nil {
		return nil, err
	}
	exe, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("resolve own executable: %w", err)
	}
	s := &ConvertService{
		dirs:   dirs,
		store:  settings.NewStore(dirs.SettingsFile()),
		prices: pricing.NewFetcher(),
	}
	cfg, _ := s.store.Load()
	s.runner = &convrunner.Runner{
		Exe:            exe,
		WorkDir:        dirs.Work(),
		CacheDir:       s.cacheDir(cfg),
		RunsDir:        dirs.Runs(),
		LexiconsDir:    s.lexiconsDir(cfg),
		EmbalignPython: cfg.EmbalignPython,
		EmbalignScript: cfg.EmbalignScript,
	}
	s.manager = jobs.NewManager(jobs.NewStore(dirs.JobsFile()), s.runner, s.resolveSpec, jobs.Hooks{
		OnState:    s.emitState,
		OnProgress: s.emitProgress,
		OnCost:     s.emitCost,
	})
	return s, nil
}

// ServiceStartup is Wails' service lifecycle hook: the queue worker starts
// with the app and dies with it.
func (s *ConvertService) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	return s.manager.Start(ctx)
}

// --- settings ---------------------------------------------------------------

func (s *ConvertService) GetSettings() (settings.Settings, error) {
	return s.store.Load()
}

func (s *ConvertService) SaveSettings(cfg settings.Settings) error {
	if err := s.store.Save(cfg); err != nil {
		return err
	}
	// Runner paths follow the saved settings; safe because only the queue
	// worker reads them and it re-reads per launch via resolveSpec paths.
	s.runner.CacheDir = s.cacheDir(cfg)
	s.runner.LexiconsDir = s.lexiconsDir(cfg)
	s.runner.EmbalignPython = cfg.EmbalignPython
	s.runner.EmbalignScript = cfg.EmbalignScript
	return nil
}

func (s *ConvertService) cacheDir(cfg settings.Settings) string {
	if cfg.CacheDir != "" {
		return cfg.CacheDir
	}
	return s.dirs.Cache()
}

func (s *ConvertService) lexiconsDir(cfg settings.Settings) string {
	if cfg.LexiconsDir != "" {
		return cfg.LexiconsDir
	}
	return s.dirs.LexiconsDir()
}

// TestProvider runs the setup dialog's connection check for one provider.
func (s *ConvertService) TestProvider(name string) keycheck.Result {
	cfg, _ := s.store.Load()
	p := cfg.Providers[name]
	ctx := context.Background()
	switch name {
	case "openrouter":
		return keycheck.CheckOpenRouter(ctx, p.BaseURL, p.APIKey)
	case "gonka":
		return keycheck.CheckGonka(ctx, p.BaseURL, p.APIKey, p.Model)
	case "claude":
		res := keycheck.CheckClaudeCLI(ctx, cfg.ClaudeBin)
		if res.ResolvedBin != "" && res.ResolvedBin != cfg.ClaudeBin {
			cfg.ClaudeBin = res.ResolvedBin
			_ = s.store.Save(cfg)
		}
		return res
	case "ollama":
		return keycheck.CheckOllamaServer(ctx, p.BaseURL, p.Model)
	case "llamacpp":
		return keycheck.CheckLlamaCpp(ctx, p.BaseURL, p.APIKey, p.Model)
	}
	return keycheck.Result{Status: keycheck.StatusUnreachable, Detail: "unknown provider " + name}
}

// Preflight is the cheap pre-Convert gate: is this provider set up at all?
// Network probes only for the local servers, where "running" is the setup.
func (s *ConvertService) Preflight(name string) keycheck.Result {
	cfg, _ := s.store.Load()
	p := cfg.Providers[name]
	switch name {
	case "openrouter", "gonka":
		if p.APIKey == "" {
			return keycheck.Result{Status: keycheck.StatusInvalidKey, Detail: "no API key saved"}
		}
		return keycheck.Result{Status: keycheck.StatusOK}
	case "claude", "ollama", "llamacpp":
		return s.TestProvider(name)
	}
	return keycheck.Result{Status: keycheck.StatusUnreachable, Detail: "unknown provider " + name}
}

// --- estimate & pricing -----------------------------------------------------

func (s *ConvertService) EstimateBook(path string) (*convrunner.Estimate, error) {
	return s.runner.Estimate(context.Background(), path)
}

// QuoteBook prices one provider/model choice for the whole book (all targets
// summed). repair follows the job-level resolution the runner will use.
func (s *ConvertService) QuoteBook(chars, sentences int, src string, targets []string, provider, model string, repair *bool) pricing.QuoteResult {
	promptUSD, completionUSD := pricing.DefaultPromptPrice, pricing.DefaultCompletionPrice
	if provider == "openrouter" {
		if model == "" {
			cfg, _ := s.store.Load()
			model = cfg.Providers[provider].Model
		}
		if model == "" {
			model = "google/gemini-3.1-flash-lite"
		}
		promptUSD, completionUSD = s.prices.Prices(context.Background(), model)
	}
	repairOn := repair != nil && *repair
	total := pricing.QuoteResult{Kind: "usd"}
	for _, tgt := range targets {
		q := pricing.Quote(chars, sentences, src, tgt, provider, promptUSD, completionUSD, repairOn)
		total.USD += q.USD
		total.Kind = q.Kind
	}
	total.Display = pricing.Display(total.USD, total.Kind)
	return total
}

// --- jobs -------------------------------------------------------------------

// EnqueueRequest is what the Convert button submits.
type EnqueueRequest struct {
	InputPath     string               `json:"inputPath"`
	OutputPath    string               `json:"outputPath"`
	Source        string               `json:"source"`
	Targets       []string             `json:"targets"`
	Provider      string               `json:"provider"`
	Model         string               `json:"model,omitempty"`
	LimitChapters int                  `json:"limitChapters,omitempty"`
	Repair        *bool                `json:"repair,omitempty"`
	RepairContext int                  `json:"repairContext,omitempty"`
	Judge         bool                 `json:"judge,omitempty"`
	Force         bool                 `json:"force,omitempty"`
	Estimate      *convrunner.Estimate `json:"estimate,omitempty"`
}

func (s *ConvertService) Enqueue(req EnqueueRequest) (jobs.Job, error) {
	if req.RepairContext != 0 && req.RepairContext != 2 {
		return jobs.Job{}, fmt.Errorf("proofreading context must be 0 or 2")
	}
	if err := os.MkdirAll(filepath.Dir(req.OutputPath), 0o755); err != nil {
		return jobs.Job{}, fmt.Errorf("create output folder: %w", err)
	}
	return s.manager.Enqueue(jobs.Job{
		InputPath:     req.InputPath,
		OutputPath:    req.OutputPath,
		Source:        req.Source,
		Targets:       req.Targets,
		Provider:      req.Provider,
		Model:         req.Model,
		LimitChapters: req.LimitChapters,
		Repair:        req.Repair,
		RepairContext: req.RepairContext,
		Judge:         req.Judge,
		Force:         req.Force,
		Estimate:      req.Estimate,
	})
}

func (s *ConvertService) Jobs() []jobs.Job           { return s.manager.Jobs() }
func (s *ConvertService) CancelJob(id string) error  { return s.manager.Cancel(id) }
func (s *ConvertService) PromoteJob(id string) error { return s.manager.Promote(id) }
func (s *ConvertService) RemoveJob(id string) error  { return s.manager.Remove(id) }
func (s *ConvertService) RetryJob(id string, force bool) (jobs.Job, error) {
	return s.manager.Retry(id, force)
}

// resolveSpec injects credentials at launch time; jobs.json stays key-free.
func (s *ConvertService) resolveSpec(j jobs.Job, attempt string) (convrunner.ConvertSpec, error) {
	cfg, _ := s.store.Load()
	p := cfg.Providers[j.Provider]
	spec := convrunner.ConvertSpec{
		JobID:            j.ID,
		Attempt:          attempt,
		InputPath:        j.InputPath,
		OutputPath:       j.OutputPath,
		Source:           j.Source,
		Targets:          j.Targets,
		Provider:         j.Provider,
		Model:            j.Model,
		BaseURL:          p.BaseURL,
		APIKey:           p.APIKey,
		ClaudeBin:        cfg.ClaudeBin,
		LimitChapters:    j.LimitChapters,
		Repair:           j.Repair,
		RepairContext:    j.RepairContext,
		Judge:            j.Judge,
		Force:            j.Force,
		AlignerInstalled: s.alignerInstalled(cfg),
	}
	if spec.Model == "" {
		spec.Model = p.Model
	}
	switch j.Provider {
	case "openrouter":
		if spec.APIKey == "" {
			return spec, fmt.Errorf("OpenRouter needs an API key — add one in Settings → Translation engines")
		}
	case "gonka":
		if spec.APIKey == "" {
			return spec, fmt.Errorf("Gonka needs an API key — add one in Settings → Translation engines")
		}
	}
	s.ensureLexicons(cfg, j.Source, j.Targets)
	return spec, nil
}

// ensureLexicons silently fetches the lexcheck dictionaries for new pairs.
// Best-effort with a tight budget: a missing lexicon is only a notice to the
// converter, so a slow or offline network must never delay the job further.
func (s *ConvertService) ensureLexicons(cfg settings.Settings, source string, targets []string) {
	dir := s.lexiconsDir(cfg)
	for _, tgt := range targets {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		_ = bootstrap.FetchLexicons(ctx, dir, source, tgt, "")
		cancel()
	}
}

// alignerInstalled: both embalign paths configured and present. Bootstrap
// (the guided install) fills these in; power users may point at their own venv.
func (s *ConvertService) alignerInstalled(cfg settings.Settings) bool {
	if cfg.EmbalignPython == "" || cfg.EmbalignScript == "" {
		return false
	}
	if _, err := os.Stat(cfg.EmbalignPython); err != nil {
		return false
	}
	_, err := os.Stat(cfg.EmbalignScript)
	return err == nil
}

// --- event fan-out ----------------------------------------------------------

func (s *ConvertService) emitState(j jobs.Job) {
	if j.Status != jobs.StatusRunning {
		s.mu.Lock()
		if s.trackerID == j.ID {
			s.trackerID, s.tracker = "", nil
		}
		s.mu.Unlock()
	}
	application.Get().Event.Emit(eventJobState, j)
}

func (s *ConvertService) emitProgress(jobID string, ev convrunner.ProgressEvent) {
	if ev.Phase == "" {
		return
	}
	s.mu.Lock()
	if s.trackerID != jobID {
		j, ok := s.manager.Get(jobID)
		if !ok {
			s.mu.Unlock()
			return
		}
		cfg, _ := s.store.Load()
		repairOn := j.Provider == "gonka"
		if j.Repair != nil {
			repairOn = *j.Repair
		}
		plan := phases.NewPlan(repairOn, j.Judge, s.alignerInstalled(cfg))
		s.tracker = phases.NewTracker(plan, j.Targets)
		s.trackerID = jobID
	}
	s.tracker.Update(ev.Phase, ev.Target, ev.Done, ev.Total)
	percent := s.tracker.Percent()
	s.mu.Unlock()

	application.Get().Event.Emit(eventJobProgress, ProgressPayload{
		ID:      jobID,
		Phase:   ev.Phase,
		Target:  ev.Target,
		Done:    ev.Done,
		Total:   ev.Total,
		Percent: percent,
		Label:   phases.Label(ev.Phase),
	})
}

func (s *ConvertService) emitCost(jobID string, usd float64) {
	application.Get().Event.Emit(eventJobCost, CostPayload{ID: jobID, CostUSD: usd})
}

// --- files & OS integration -------------------------------------------------

func (s *ConvertService) PickBook() (string, error) {
	return application.Get().Dialog.OpenFile().
		SetTitle("Choose a book").
		AddFilter("Books", "*.epub;*.fb2;*.fb2.zip;*.tbook").
		PromptForSingleSelection()
}

func (s *ConvertService) PickDirectory(title string) (string, error) {
	return application.Get().Dialog.OpenFile().
		SetTitle(title).
		CanChooseDirectories(true).
		CanChooseFiles(false).
		PromptForSingleSelection()
}

// SuggestOutputPath builds "<Title> [src-tgt1,tgt2].tbook" inside the user's
// output folder (or next to the input when none is set).
func (s *ConvertService) SuggestOutputPath(inputPath, title, source string, targets []string) string {
	cfg, _ := s.store.Load()
	dir := cfg.OutputDir
	if dir == "" {
		dir = paths.DefaultOutputDir()
	}
	if dir == "" {
		dir = filepath.Dir(inputPath)
	}
	base := strings.TrimSpace(title)
	if base == "" {
		base = strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
	}
	base = sanitizeFilename(base)
	return filepath.Join(dir, fmt.Sprintf("%s [%s-%s].tbook", base, source, strings.Join(targets, ",")))
}

func sanitizeFilename(name string) string {
	repl := strings.NewReplacer("/", "-", "\\", "-", ":", "-", "*", "", "?", "", "\"", "'", "<", "(", ">", ")", "|", "-")
	name = repl.Replace(name)
	if len(name) > 120 {
		name = name[:120]
	}
	return strings.TrimSpace(name)
}

// RevealInFolder opens the OS file manager with the file selected.
func (s *ConvertService) RevealInFolder(path string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", "-R", path).Start()
	case "windows":
		return exec.Command("explorer", "/select,", path).Start()
	default:
		// No cross-DE "select file" convention: open the containing folder.
		return exec.Command("xdg-open", filepath.Dir(path)).Start()
	}
}

func (s *ConvertService) OpenExternal(url string) error {
	return application.Get().Browser.OpenURL(url)
}

// --- meta -------------------------------------------------------------------

type AppInfo struct {
	Version          string `json:"version"`
	Commit           string `json:"commit"`
	Platform         string `json:"platform"`
	AlignerInstalled bool   `json:"alignerInstalled"`
	DefaultOutputDir string `json:"defaultOutputDir"`
}

func (s *ConvertService) Info() AppInfo {
	cfg, _ := s.store.Load()
	return AppInfo{
		Version:          buildinfo.Version,
		Commit:           buildinfo.Commit(),
		Platform:         runtime.GOOS,
		AlignerInstalled: s.alignerInstalled(cfg),
		DefaultOutputDir: paths.DefaultOutputDir(),
	}
}
