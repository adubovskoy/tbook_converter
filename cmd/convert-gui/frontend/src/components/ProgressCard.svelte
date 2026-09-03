<script lang="ts">
  import type { Job, ProgressPayload } from "../lib/api";
  import { copy } from "../lib/copy";
  import { langName } from "../lib/lang";
  import { fmtInt } from "../lib/types";

  let {
    job,
    progress = undefined,
    targetProgress = {},
  }: {
    job: Job;
    progress?: ProgressPayload;
    targetProgress?: Record<string, ProgressPayload>;
  } = $props();

  const pct = $derived(Math.min(100, Math.max(0, Math.round(progress?.percent ?? 0))));
  const started = $derived(!!progress);
</script>

<div class="card">
  <div class="progress-row">
    <div class="progress-track big">
      <div class="progress-fill" class:indeterminate={!started} style:width="{pct}%"></div>
    </div>
    <span class="progress-pct">{started ? `${pct}%` : ""}</span>
  </div>
  {#if progress}
    <div class="progress-label">
      {progress.label}
      {#if progress.total > 0}
        <span class="progress-counts nums">
          — {fmtInt(progress.done)} of {fmtInt(progress.total)}</span>
      {/if}
    </div>
  {:else}
    <div class="progress-label">{copy.ui.starting}</div>
  {/if}

  {#if job.targets.length > 1}
    <div class="target-rows">
      {#each job.targets as t (t)}
        {@const tp = targetProgress[t]}
        <div class="target-row">
          <span class="lang">{langName(t)}</span>
          {#if tp}
            <div class="progress-track">
              <div
                class="progress-fill"
                style:width="{tp.total > 0 ? Math.round((tp.done / tp.total) * 100) : 0}%"
              ></div>
            </div>
            <span class="nums">{tp.label} · {fmtInt(tp.done)}/{fmtInt(tp.total)}</span>
          {:else}
            <span>{copy.ui.waiting}</span>
          {/if}
        </div>
      {/each}
    </div>
  {/if}
</div>
