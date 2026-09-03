import type { Estimate } from "./api";

export type View = "setup" | "convert" | "job" | "jobs" | "settings";

export type ConvertStage = "pick" | "estimating" | "configure";

/** The Convert screen's state lives in the shared app class so a detour into
 *  the setup wizard (or any other screen) never loses the user's choices. */
export interface ConvertState {
  stage: ConvertStage;
  inputPath: string;
  isTbook: boolean;
  estimate: Estimate | null;
  source: string;
  targets: string[];
  provider: string;
  model: string;
  /** User-chosen output folder; "" follows settings/default via Go. */
  outputDir: string;
  outputPath: string;
  limitChapters: number; // 0 = full book
  repair: boolean | null; // null = provider default (Auto)
  repairContext: number; // 0 | 2 only (--context 1 is a CLI hard error)
  judge: boolean;
  force: boolean;
}

export function newConvertState(): ConvertState {
  return {
    stage: "pick",
    inputPath: "",
    isTbook: false,
    estimate: null,
    source: "",
    targets: [],
    provider: "",
    model: "",
    outputDir: "",
    outputPath: "",
    limitChapters: 0,
    repair: null,
    repairContext: 0,
    judge: false,
    force: false,
  };
}

// --- small shared helpers ----------------------------------------------------

/** JSON round-trip: binding class instances → plain objects that Svelte's
 *  $state proxies deeply (class instances are not made reactive). */
export function plain<T>(v: T): T {
  return JSON.parse(JSON.stringify(v)) as T;
}

export function basename(p: string): string {
  const i = Math.max(p.lastIndexOf("/"), p.lastIndexOf("\\"));
  return i >= 0 ? p.slice(i + 1) : p;
}

export function dirname(p: string): string {
  const i = Math.max(p.lastIndexOf("/"), p.lastIndexOf("\\"));
  return i > 0 ? p.slice(0, i) : p;
}

export function joinPath(dir: string, name: string): string {
  const sep = dir.includes("\\") ? "\\" : "/";
  return dir.endsWith(sep) ? dir + name : dir + sep + name;
}

export function fmtUSD(v: number): string {
  return `$${v.toFixed(2)}`;
}

export function fmtInt(v: number): string {
  return v.toLocaleString("en-US");
}

export function fmtDate(iso: string): string {
  const d = new Date(iso);
  if (isNaN(d.getTime())) return "";
  return d.toLocaleDateString("en-US", { month: "short", day: "numeric", year: "numeric" });
}

export function fmtElapsed(ms: number): string {
  const s = Math.max(0, Math.floor(ms / 1000));
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  const sec = s % 60;
  const mm = String(m).padStart(2, "0");
  const ss = String(sec).padStart(2, "0");
  return h > 0 ? `${h}:${mm}:${ss}` : `${m}:${ss.padStart(2, "0")}`;
}
