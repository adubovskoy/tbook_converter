<script lang="ts">
  import type { Estimate } from "../lib/api";
  import { copy } from "../lib/copy";
  import { langName } from "../lib/lang";
  import { fmtInt } from "../lib/types";

  let { estimate }: { estimate: Estimate } = $props();
</script>

<div class="card">
  <div class="card-title">{estimate.title || copy.ui.untitled}</div>
  {#if estimate.author}
    <div class="estimate-author">{copy.ui.by} {estimate.author}</div>
  {/if}
  <div class="facts">
    <div class="fact">
      <span class="fact-num">
        {estimate.detectedLanguage ? langName(estimate.detectedLanguage) : copy.ui.factUnknown}
      </span>
      <span class="fact-label">{copy.ui.factLanguage}</span>
    </div>
    <div class="fact">
      <span class="fact-num">{fmtInt(estimate.chapters)}</span>
      <span class="fact-label">{copy.ui.factChapters}</span>
    </div>
    <div class="fact">
      <span class="fact-num">{fmtInt(estimate.words)}</span>
      <span class="fact-label">{copy.ui.factWords}</span>
    </div>
    <div class="fact">
      <span class="fact-num">{fmtInt(estimate.sentences)}</span>
      <span class="fact-label">{copy.ui.factSentences}</span>
    </div>
  </div>
  {#if estimate.noteSentences > 0}
    <div class="fact-note">{copy.ui.footnoteSentences(fmtInt(estimate.noteSentences))}</div>
  {/if}
</div>
