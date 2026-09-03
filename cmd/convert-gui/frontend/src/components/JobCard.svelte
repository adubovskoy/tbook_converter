<script lang="ts">
  import { PromoteJob, RemoveJob, RetryJob, RevealInFolder, type Job } from "../lib/api";
  import { app, jobProviderName, jobTitle } from "../lib/app.svelte";
  import { copy } from "../lib/copy";
  import { langName } from "../lib/lang";
  import { fmtDate, fmtUSD, plain } from "../lib/types";
  import ConfirmSheet from "./ConfirmSheet.svelte";

  let { job }: { job: Job } = $props();

  let menuOpen = $state(false);
  let confirmRemove = $state(false);

  const statusLabel = copy.ui.status;

  const meta = $derived.by(() => {
    const parts = [
      `${langName(job.source || "?")} → ${job.targets.map(langName).join(", ")}`,
      jobProviderName(job),
      fmtDate(job.createdAt),
    ];
    if (job.costUSD > 0.001) parts.push(fmtUSD(job.costUSD));
    return parts.filter(Boolean).join(" · ");
  });

  const pct = $derived(Math.round(app.progress[job.id]?.percent ?? 0));

  async function resume() {
    app.upsertJob(plain(await RetryJob(job.id, false)));
  }
  async function startNow() {
    await PromoteJob(job.id);
  }
  async function remove() {
    confirmRemove = false;
    await RemoveJob(job.id);
    app.jobs = app.jobs.filter((j) => j.id !== job.id);
  }
  function addLanguage() {
    menuOpen = false;
    app.openBook(job.outputPath);
  }
  function convertAgain() {
    menuOpen = false;
    app.openBook(job.inputPath);
  }
</script>

<svelte:window
  onclick={() => {
    if (menuOpen) menuOpen = false;
  }}
/>

<div class="job-card">
  <button class="job-open" onclick={() => app.openJob(job.id)}>
    <div class="job-title">{jobTitle(job)}</div>
    <div class="job-sub">{meta}</div>
  </button>

  <span class="job-status {job.status}">
    {statusLabel[job.status] ?? job.status}{job.status === "running" ? ` · ${pct}%` : ""}
  </span>

  <div class="job-actions">
    {#if job.status === "queued"}
      <button class="btn" onclick={startNow}>{copy.job.startNow}</button>
      <button class="text-btn" onclick={() => (confirmRemove = true)}>{copy.job.remove}</button>
    {:else if job.status === "done"}
      <button class="btn" onclick={() => RevealInFolder(job.outputPath)}>{copy.job.showInFolder}</button>
      <div class="menu-wrap">
        <button
          class="icon-btn"
          aria-label="More actions"
          onclick={(e) => {
            e.stopPropagation();
            menuOpen = !menuOpen;
          }}>⋯</button
        >
        {#if menuOpen}
          <div class="menu">
            <button onclick={addLanguage}>{copy.jobs.addLanguage}</button>
            <button onclick={convertAgain}>{copy.jobs.convertAgain}</button>
            <button
              class="danger"
              onclick={() => {
                menuOpen = false;
                confirmRemove = true;
              }}>{copy.jobs.removeFromList}</button
            >
          </div>
        {/if}
      </div>
    {:else if job.status === "failed" || job.status === "canceled" || job.status === "interrupted"}
      <button class="btn primary" onclick={resume}>{copy.job.resume}</button>
      <button class="text-btn" onclick={() => (confirmRemove = true)}>{copy.job.remove}</button>
    {/if}
  </div>
</div>

{#if confirmRemove}
  <ConfirmSheet
    title={copy.jobs.removeSheetTitle}
    body={copy.jobs.removeSheetBody}
    confirmLabel={copy.jobs.removeConfirm}
    cancelLabel={copy.jobs.removeKeep}
    onconfirm={remove}
    oncancel={() => (confirmRemove = false)}
  />
{/if}
