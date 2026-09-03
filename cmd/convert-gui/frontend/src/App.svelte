<script lang="ts">
  import { app } from "./lib/app.svelte";
  import { copy } from "./lib/copy";
  import Banner from "./components/Banner.svelte";
  import Convert from "./screens/Convert.svelte";
  import JobDetail from "./screens/JobDetail.svelte";
  import Jobs from "./screens/Jobs.svelte";
  import Settings from "./screens/Settings.svelte";
  import Setup from "./screens/Setup.svelte";

  // Theme follows settings; "system" defers to prefers-color-scheme in CSS.
  $effect(() => {
    document.documentElement.dataset.theme = app.settings.ui?.theme || "system";
  });
</script>

{#if !app.booted}
  <div class="boot">{copy.ui.loading}</div>
{:else}
  {#if app.banner}
    <div class="toast-wrap">
      <Banner kind={app.banner.kind} text={app.banner.text} ondismiss={() => app.dismissBanner()} />
    </div>
  {/if}

  {#if app.view === "setup"}
    <Setup />
  {:else if app.view === "convert"}
    <Convert />
  {:else if app.view === "job"}
    <JobDetail />
  {:else if app.view === "jobs"}
    <Jobs />
  {:else}
    <Settings />
  {/if}
{/if}
