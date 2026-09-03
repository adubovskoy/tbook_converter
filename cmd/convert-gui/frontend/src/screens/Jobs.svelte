<script lang="ts">
  import { app } from "../lib/app.svelte";
  import { copy } from "../lib/copy";
  import AppBar from "../components/AppBar.svelte";
  import JobCard from "../components/JobCard.svelte";

  const running = $derived(app.jobs.filter((j) => j.status === "running"));
  const queued = $derived(app.queuedJobs);
  const history = $derived(
    app.jobs
      .filter((j) => j.status !== "running" && j.status !== "queued")
      .sort((a, b) => Date.parse(b.createdAt) - Date.parse(a.createdAt)),
  );
</script>

<div class="screen">
  <AppBar title={copy.jobs.title} back="convert" />

  <div class="page">
    {#if app.jobs.length === 0}
      <p class="empty-list">{copy.jobs.empty}</p>
    {:else}
      {#if running.length > 0}
        <div class="job-section">{copy.jobs.running}</div>
        {#each running as job (job.id)}
          <JobCard {job} />
        {/each}
      {/if}

      {#if queued.length > 0}
        <div class="job-section">{copy.jobs.queued}</div>
        {#each queued as job (job.id)}
          <JobCard {job} />
        {/each}
      {/if}

      {#if history.length > 0}
        <div class="job-section">{copy.jobs.history}</div>
        {#each history as job (job.id)}
          <JobCard {job} />
        {/each}
      {/if}
    {/if}
  </div>
</div>
