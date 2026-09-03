<script lang="ts">
  import { OpenExternal } from "../lib/api";
  import { HOWTO, type ProviderId } from "../lib/copy";

  let { provider }: { provider: ProviderId } = $props();

  const h = $derived(HOWTO[provider]);
</script>

<div class="howto">
  <div class="howto-title">{h.title}</div>
  <ol class="howto-steps">
    {#each h.steps as s, i (i)}
      <li>
        {s.text}
        {#if s.url}
          <button class="link" onclick={() => OpenExternal(s.url!)}>{s.linkLabel ?? s.url}</button>
        {/if}
        {#if s.code}
          <code>{s.code}</code>
        {/if}
      </li>
    {/each}
  </ol>
  {#if h.note}
    <p class="howto-note">{h.note}</p>
  {/if}
</div>
