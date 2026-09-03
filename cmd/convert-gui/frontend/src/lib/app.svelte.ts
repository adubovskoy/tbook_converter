import { Events } from "@wailsio/runtime";
import * as api from "./api";
import type { AppInfo, Job, ProgressPayload, Settings } from "./api";
import { newConvertState, plain, type ConvertState, type View } from "./types";
import { copy, providerName, type ProviderId } from "./copy";

/** Shared reactive app state: settings, jobs, events, navigation. */
class AppState {
  booted = $state(false);
  view = $state<View>("convert");

  settings = $state<Settings>(emptySettings());
  info = $state<AppInfo>({
    version: "",
    commit: "",
    platform: "",
    alignerInstalled: false,
    defaultOutputDir: "",
  });
  jobs = $state<Job[]>([]);

  /** JobDetail target. */
  currentJobId = $state("");
  /** Latest overall progress tick per job id. */
  progress = $state<Record<string, ProgressPayload>>({});
  /** Latest tick per target per job id (multi-target mini-rows). */
  targetProgress = $state<Record<string, Record<string, ProgressPayload>>>({});

  /** Transient global notice (floats under the app bar). */
  banner = $state<{ kind: "success" | "info" | "danger"; text: string } | null>(null);
  #bannerTimer: ReturnType<typeof setTimeout> | undefined;

  // Setup wizard entry state (settable from Convert/Settings deep links).
  setupStep = $state(1);
  setupProvider = $state<ProviderId | "">("");
  setupReturn = $state<View>("convert");

  convert = $state<ConvertState>(newConvertState());

  runningJob = $derived(this.jobs.find((j) => j.status === "running"));
  queuedJobs = $derived(this.jobs.filter((j) => j.status === "queued"));
  queueCount = $derived(
    this.jobs.filter((j) => j.status === "running" || j.status === "queued").length,
  );

  async init() {
    if (this.booted) return;

    Events.On("job:state", (ev) => {
      this.upsertJob(plain(ev.data));
    });
    Events.On("job:progress", (ev) => {
      const p = plain(ev.data);
      this.progress[p.id] = p;
      if (p.target) {
        const m = this.targetProgress[p.id] ?? {};
        m[p.target] = p;
        this.targetProgress[p.id] = m;
      }
    });
    Events.On("job:cost", (ev) => {
      const j = this.jobs.find((x) => x.id === ev.data.id);
      if (j) j.costUSD = ev.data.costUSD;
    });

    try {
      const [cfg, jobs, info] = await Promise.all([api.GetSettings(), api.Jobs(), api.Info()]);
      this.settings = plain(cfg);
      this.jobs = plain(jobs ?? []);
      this.info = plain(info);
    } catch (e) {
      console.error("boot failed", e);
    }
    this.view =
      !this.settings.setupCompleted && !this.hasAnyProvider() ? "setup" : "convert";
    this.booted = true;
  }

  /** Any engine already usable (key saved, Claude Code resolved, local set up)? */
  hasAnyProvider(): boolean {
    const p = this.settings.providers ?? {};
    if (p["openrouter"]?.apiKey || p["gonka"]?.apiKey) return true;
    if (this.settings.claudeBin) return true;
    return "ollama" in p || "llamacpp" in p;
  }

  /** Is one specific engine set up enough to offer on the Convert screen? */
  providerConfigured(name: string): boolean {
    const p = this.settings.providers ?? {};
    switch (name) {
      case "openrouter":
      case "gonka":
        return !!p[name]?.apiKey;
      case "claude":
        return !!this.settings.claudeBin;
      default:
        // Local servers have no key; a completed wizard step marks them known.
        return name in p;
    }
  }

  upsertJob(j: Job) {
    const i = this.jobs.findIndex((x) => x.id === j.id);
    if (i >= 0) this.jobs[i] = j;
    else this.jobs.push(j); // append: queued section must read in FIFO order
    if (j.status !== "running" && j.status !== "queued") {
      // Terminal: drop stale ticks so a retry starts with a clean bar.
      if (j.status !== "done") delete this.progress[j.id];
    }
  }

  async saveSettings() {
    const snap = $state.snapshot(this.settings) as Settings;
    await api.SaveSettings(snap);
  }

  async refreshInfo() {
    try {
      this.info = plain(await api.Info());
    } catch {
      /* non-fatal */
    }
  }

  // --- navigation -------------------------------------------------------------

  go(v: View) {
    this.view = v;
  }

  openJob(id: string) {
    this.currentJobId = id;
    this.view = "job";
  }

  openSetup(step: number, provider: ProviderId | "" = "", returnTo: View = "convert") {
    this.setupStep = step;
    this.setupProvider = provider;
    this.setupReturn = returnTo;
    this.view = "setup";
  }

  showBanner(text: string, kind: "success" | "info" | "danger" = "success") {
    this.banner = { kind, text };
    clearTimeout(this.#bannerTimer);
    this.#bannerTimer = setTimeout(() => (this.banner = null), 6000);
  }

  dismissBanner() {
    clearTimeout(this.#bannerTimer);
    this.banner = null;
  }

  // --- convert flow -----------------------------------------------------------

  /** Start a conversion from a picked file (also used by "Add another language"
   *  and "Convert again", which prefill the path). */
  async openBook(path: string) {
    this.convert = newConvertState();
    this.view = "convert";
    const c = this.convert;
    c.inputPath = path;
    c.targets = (this.settings.defaultTargets ?? []).slice();
    c.repair = this.settings.repair ?? null;
    c.repairContext = this.settings.repairContext === 2 ? 2 : 0;
    c.judge = this.settings.judge ?? false;
    c.provider = this.defaultConfiguredProvider();
    c.model = c.provider === "openrouter" ? this.settings.providers?.["openrouter"]?.model || "google/gemini-3.7-flash" : "";

    if (path.toLowerCase().endsWith(".tbook")) {
      // EstimateBook rejects .tbook — the archive itself is authoritative for
      // the source language, so the converter infers it and we pass "".
      c.isTbook = true;
      c.stage = "configure";
      return;
    }

    c.stage = "estimating";
    try {
      const est = await api.EstimateBook(path);
      if (!est) throw new Error("no estimate returned");
      const e = plain(est);
      c.estimate = e;
      c.source = e.detectedLanguage || this.settings.defaultSource || "";
      c.targets = c.targets.filter((t) => t !== c.source);
      c.stage = "configure";
    } catch (err) {
      this.convert = newConvertState();
      this.showBanner(`${copy.convert.estimateFailed}: ${errMsg(err)}`, "danger");
    }
  }

  defaultConfiguredProvider(): string {
    const d = this.settings.defaultProvider;
    if (d && this.providerConfigured(d)) return d;
    for (const p of ["openrouter", "gonka", "claude", "ollama", "llamacpp"]) {
      if (this.providerConfigured(p)) return p;
    }
    return "";
  }
}

function emptySettings(): Settings {
  // A plain-object copy of the binding model's zero value: $state proxies
  // POJOs deeply, class instances not.
  return plain(new api.Settings());
}

export function errMsg(err: unknown): string {
  if (err instanceof Error) return err.message;
  return String(err);
}

export function jobTitle(j: Job): string {
  if (j.estimate?.title) return j.estimate.title;
  const b = j.inputPath.split(/[\\/]/).pop() ?? j.inputPath;
  return b.replace(/\.(epub|fb2|fb2\.zip|tbook)$/i, "");
}

export function jobProviderName(j: Job): string {
  return providerName(j.provider);
}

export const app = new AppState();
