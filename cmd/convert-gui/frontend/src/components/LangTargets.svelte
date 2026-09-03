<script lang="ts">
  import { ALL_LANGS, langName, QUICK_TARGETS } from "../lib/lang";
  import { copy } from "../lib/copy";

  let {
    selected = $bindable([]),
    source = "",
    defaults = [],
    onchange = () => {},
  }: {
    selected?: string[];
    source?: string;
    defaults?: string[];
    onchange?: () => void;
  } = $props();

  let moreOpen = $state(false);

  /** Inline chips: defaults first, then the quick list, plus anything already
   *  selected from the full sheet — deduped, source excluded. */
  const quick = $derived.by(() => {
    const seen = new Set<string>();
    const out: string[] = [];
    for (const code of [...defaults, ...QUICK_TARGETS, ...selected]) {
      if (code === source || seen.has(code)) continue;
      seen.add(code);
      out.push(code);
    }
    return out;
  });

  function toggle(code: string) {
    if (code === source) return;
    selected = selected.includes(code) ? selected.filter((c) => c !== code) : [...selected, code];
    onchange();
  }
</script>

<div class="chips">
  {#each quick as code (code)}
    <button class="chip" class:selected={selected.includes(code)} onclick={() => toggle(code)}>
      {langName(code)}
    </button>
  {/each}
  <button class="chip" onclick={() => (moreOpen = true)}>{copy.convert.more}</button>
</div>

{#if moreOpen}
  <div class="sheet-backdrop">
    <div class="sheet" role="dialog" aria-label={copy.ui.allLanguages}>
      <div class="sheet-handle"></div>
      <div class="sheet-title">{copy.ui.allLanguages}</div>
      <div class="chips">
        {#each ALL_LANGS as code (code)}
          <button
            class="chip"
            class:selected={selected.includes(code)}
            disabled={code === source}
            onclick={() => toggle(code)}
          >
            {langName(code)}
          </button>
        {/each}
      </div>
      <div class="sheet-actions" style="margin-top:18px">
        <button class="btn primary" onclick={() => (moreOpen = false)}>{copy.ui.done}</button>
      </div>
    </div>
  </div>
{/if}
