<script lang="ts">
  import type { QuoteResult } from "../lib/api";
  import { copy, OPENROUTER_MODELS, PROVIDER_CARDS, PROVIDER_NAMES, type ProviderId } from "../lib/copy";

  let {
    provider = $bindable(""),
    model = $bindable(""),
    quotes = {},
    configured,
    showPrices = true,
    onsetup,
    onchange = () => {},
  }: {
    provider?: string;
    model?: string;
    /** keyed `${provider}|${model}` */
    quotes?: Record<string, QuoteResult>;
    configured: (name: string) => boolean;
    showPrices?: boolean;
    onsetup: (name: ProviderId) => void;
    onchange?: () => void;
  } = $props();

  const cardBlurb = (id: string) => PROVIDER_CARDS.find((c) => c.id === id)?.blurb ?? "";

  function price(p: string, m: string): string {
    if (!showPrices) return copy.convert.priceUnknown;
    return quotes[`${p}|${m}`]?.display ?? "…";
  }

  function pick(p: string, m: string) {
    provider = p;
    model = m;
    onchange();
  }

  const simple: ProviderId[] = ["gonka", "claude", "ollama", "llamacpp"];
</script>

<div class="provider-rows">
  {#if configured("openrouter")}
    <div class="provider-row" style="border-bottom:none; padding-bottom:4px">
      <span style="width:18px"></span>
      <span class="provider-main">
        <span class="provider-name">
          OpenRouter <span class="badge">Recommended</span>
        </span>
        <span class="provider-desc">{cardBlurb("openrouter")}</span>
      </span>
    </div>
    {#each OPENROUTER_MODELS as m (m.id)}
      <button
        class="provider-row sub"
        class:selected={provider === "openrouter" && model === m.id}
        onclick={() => pick("openrouter", m.id)}
      >
        <span class="radio-dot"></span>
        <span class="provider-main">
          <span class="provider-name">{m.label}</span>
          <span class="provider-desc">{m.desc}</span>
        </span>
        <span class="provider-price">{price("openrouter", m.id)}</span>
      </button>
    {/each}
  {:else}
    <div class="provider-row dim">
      <span style="width:18px"></span>
      <span class="provider-main">
        <span class="provider-name">
          OpenRouter <span class="badge">Recommended</span>
        </span>
        <span class="provider-desc">{cardBlurb("openrouter")}</span>
      </span>
      <button class="link" onclick={() => onsetup("openrouter")}>{copy.convert.setUp}</button>
    </div>
  {/if}

  {#each simple as p (p)}
    {#if configured(p)}
      <button class="provider-row" class:selected={provider === p} onclick={() => pick(p, "")}>
        <span class="radio-dot"></span>
        <span class="provider-main">
          <span class="provider-name">{PROVIDER_NAMES[p]}</span>
          <span class="provider-desc">{cardBlurb(p) || copy.ui.llamacppBlurb}</span>
        </span>
        <span class="provider-price">{price(p, "")}</span>
      </button>
    {:else}
      <div class="provider-row dim">
        <span style="width:18px"></span>
        <span class="provider-main">
          <span class="provider-name">{PROVIDER_NAMES[p]}</span>
          <span class="provider-desc">{cardBlurb(p) || copy.ui.llamacppBlurb}</span>
        </span>
        <button class="link" onclick={() => onsetup(p)}>{copy.convert.setUp}</button>
      </div>
    {/if}
  {/each}
</div>
