<script lang="ts">
  import { CancelJob, PromoteJob, RemoveJob, RetryJob, RevealInFolder } from "../lib/api";
  import { app, jobProviderName, jobTitle } from "../lib/app.svelte";
  import { copy } from "../lib/copy";
  import { langName } from "../lib/lang";
  import { fmtElapsed, fmtUSD, plain } from "../lib/types";
  import AppBar from "../components/AppBar.svelte";
  import ConfirmSheet from "../components/ConfirmSheet.svelte";
  import ProgressCard from "../components/ProgressCard.svelte";

  const job = $derived(app.jobs.find((j) => j.id === app.currentJobId));

  let confirmCancel = $state(false);
  let now = $state(Date.now());

  $effect(() => {
    if (job?.status !== "running") return;
    const t = setInterval(() => (now = Date.now()), 1000);
    return () => clearInterval(t);
  });

  const elapsed = $derived.by(() => {
    if (!job?.startedAt) return "";
    return fmtElapsed(now - Date.parse(job.startedAt));
  });

  const costLine = $derived.by(() => {
    if (!job) return "";
    switch (job.provider) {
      case "openrouter":
        return `${copy.job.spentSoFar} ${fmtUSD(job.costUSD)}`;
      case "gonka":
        return job.costUSD < 0.005
          ? `${copy.job.spentSoFar} ${copy.job.underCent}`
          : `${copy.job.spentSoFar} ${fmtUSD(job.costUSD)}`;
      case "claude":
        return copy.job.claudeCost;
      default:
        return copy.job.localCost;
    }
  });

  const langsLine = $derived(
    job ? `${langName(job.source || "?")} → ${job.targets.map(langName).join(", ")}` : "",
  );

  /** First meaningful line of the error, clipped, with the Claude usage-limit
   *  case replaced by a friendly explanation. */
  const errorExcerpt = $derived.by(() => {
    const e = job?.error ?? "";
    if (!e) return "";
    if (e.includes("UsageLimitError")) return copy.job.usageLimit;
    const line = e.split("\n").find((l) => l.trim() !== "") ?? e;
    return line.length > 220 ? line.slice(0, 220) + "…" : line;
  });

  async function cancel() {
    confirmCancel = false;
    if (job) await CancelJob(job.id);
  }
  async function retry(force: boolean) {
    if (!job) return;
    app.upsertJob(plain(await RetryJob(job.id, force)));
  }
  async function remove() {
    if (!job) return;
    await RemoveJob(job.id);
    app.jobs = app.jobs.filter((j) => j.id !== job.id);
    app.go("jobs");
  }
  function convertAnother() {
    app.go("convert");
  }
</script>

<div class="screen">
  <AppBar title={job ? jobTitle(job) : copy.job.title} back="jobs" />

  <div class="page narrow">
    {#if !job}
      <p class="empty-list">{copy.job.notFound}</p>
    {:else if job.status === "queued"}
      <div class="card">
        <div class="card-title">{copy.job.queuedTitle}</div>
        <p class="card-sub">{copy.job.queuedBody}</p>
        <div class="job-meta-line">
          <span>{langsLine}</span>
          <span>{jobProviderName(job)}</span>
        </div>
        <div class="row-actions">
          <button class="btn primary" onclick={() => PromoteJob(job.id)}>{copy.job.startNow}</button>
          <button class="btn" onclick={remove}>{copy.job.remove}</button>
        </div>
      </div>
    {:else if job.status === "running"}
      <ProgressCard
        {job}
        progress={app.progress[job.id]}
        targetProgress={app.targetProgress[job.id] ?? {}}
      />
      <div class="job-meta-line">
        <span>{langsLine}</span>
        <span>{jobProviderName(job)}</span>
        <span class="nums">{costLine}</span>
        {#if elapsed}
          <span class="nums">{copy.job.elapsed} {elapsed}</span>
        {/if}
      </div>
      <p class="reassure">{copy.job.runningReassure}</p>
      <div class="row-actions">
        <button class="btn" onclick={() => (confirmCancel = true)}>{copy.job.cancel}</button>
      </div>
    {:else if job.status === "done"}
      <div class="card" style="text-align:center; padding:32px 24px">
        <div class="big-icon">✅</div>
        <div class="card-title">{copy.job.doneTitle}</div>
        <p class="done-path">{job.outputPath}</p>
        {#if job.costUSD > 0.001}
          <p class="card-sub nums">{copy.job.totalCost} {fmtUSD(job.costUSD)}</p>
        {/if}
        <div class="row-actions" style="justify-content:center">
          <button class="btn primary" onclick={() => RevealInFolder(job.outputPath)}>
            {copy.job.showInFolder}
          </button>
          <button class="btn" onclick={convertAnother}>{copy.job.convertAnother}</button>
        </div>
        <p class="reassure">{copy.job.readerHint}</p>
      </div>
    {:else if job.status === "failed"}
      <div class="card">
        <div class="card-title">{copy.job.failedTitle}</div>
        <p style="font-size:14px; line-height:1.5">{errorExcerpt}</p>
        {#if job.error}
          <details class="tech">
            <summary>{copy.job.techDetails}</summary>
            <pre>{job.error}</pre>
          </details>
        {/if}
        <div class="row-actions">
          <button class="btn primary" onclick={() => retry(false)}>{copy.job.retry}</button>
          <button class="text-btn" onclick={() => retry(true)}>{copy.job.startOver}</button>
        </div>
        <p class="reassure">{copy.job.retryHint}</p>
      </div>
    {:else}
      <!-- canceled | interrupted -->
      <div class="card">
        <div class="card-title">
          {job.status === "canceled" ? copy.job.canceledTitle : copy.job.interruptedTitle}
        </div>
        <p class="card-sub">
          {job.status === "canceled" ? copy.job.canceledBody : copy.job.interruptedBody}
        </p>
        <div class="row-actions">
          <button class="btn primary" onclick={() => retry(false)}>{copy.job.resume}</button>
          <button class="btn" onclick={remove}>{copy.job.remove}</button>
        </div>
      </div>
    {/if}
  </div>
</div>

{#if confirmCancel}
  <ConfirmSheet
    title={copy.job.cancelSheetTitle}
    body={copy.job.cancelSheetBody}
    confirmLabel={copy.job.cancelConfirm}
    cancelLabel={copy.job.cancelKeep}
    onconfirm={cancel}
    oncancel={() => (confirmCancel = false)}
  />
{/if}
