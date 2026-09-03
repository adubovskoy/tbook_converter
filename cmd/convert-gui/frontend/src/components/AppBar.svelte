<script lang="ts">
  import { app, jobTitle } from "../lib/app.svelte";
  import { copy } from "../lib/copy";
  import type { View } from "../lib/types";

  let {
    title,
    back = null,
  }: {
    title: string;
    /** View the back arrow returns to; null hides the arrow. */
    back?: View | null;
  } = $props();

  const running = $derived(app.runningJob);
  const pct = $derived(running ? Math.round(app.progress[running.id]?.percent ?? 0) : 0);
</script>

<header class="appbar">
  {#if back}
    <button class="icon-btn" title={copy.ui.back} onclick={() => app.go(back!)}>←</button>
  {/if}
  <h1 class="appbar-title">{title}</h1>
  <div class="appbar-actions">
    {#if running && !(app.view === "job" && app.currentJobId === running.id)}
      <button class="run-pill" onclick={() => app.openJob(running.id)}>
        <span class="dot">●</span>
        <span class="nums">{copy.ui.convertingPill(jobTitle(running), pct)}</span>
      </button>
    {/if}
    {#if app.view !== "jobs"}
      <button class="icon-btn queue-btn" title={copy.ui.queue} onclick={() => app.go("jobs")}>
        ☰
        {#if app.queueCount > 0}
          <span class="queue-badge">{app.queueCount}</span>
        {/if}
      </button>
    {/if}
    {#if app.view !== "settings"}
      <button class="icon-btn" title={copy.ui.settings} onclick={() => app.go("settings")}>⚙</button>
    {/if}
  </div>
</header>
