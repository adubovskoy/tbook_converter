<script lang="ts">
  import { untrack } from "svelte";
  import { GetSettings, TestProvider, type KeycheckResult } from "../lib/api";
  import { app } from "../lib/app.svelte";
  import { copy, PROVIDER_CARDS, PROVIDER_NAMES, type ProviderId } from "../lib/copy";
  import { plain } from "../lib/types";
  import KeyField from "../components/KeyField.svelte";
  import ProviderHowto from "../components/ProviderHowto.svelte";

  const step = $derived(app.setupStep);
  const provider = $derived(app.setupProvider);

  // Step-3 form fields, reloaded from settings whenever the provider changes.
  let apiKey = $state("");
  let baseURL = $state("");
  let testing = $state(false);
  let result = $state<KeycheckResult | null>(null);

  $effect(() => {
    const p = provider;
    // Untracked: saving settings mid-test must not reset the form/result.
    untrack(() => {
      result = null;
      testing = false;
      const saved = p ? app.settings.providers?.[p] : undefined;
      apiKey = saved?.apiKey ?? "";
      baseURL = saved?.baseURL ?? "";
    });
  });

  const okTested = $derived(result?.status === "ok");
  const needsKey = $derived(provider === "openrouter" || provider === "gonka");
  const isLocal = $derived(provider === "ollama" || provider === "llamacpp");

  function selectProvider(p: ProviderId) {
    app.setupProvider = p;
    app.setupStep = 3;
  }

  /** TestProvider reads the *saved* settings, so persist the fields first. */
  async function mergeAndSave() {
    const providers = app.settings.providers ?? (app.settings.providers = {});
    const cur = providers[provider as string] ?? emptyProvider();
    if (needsKey) cur.apiKey = apiKey.trim();
    if (isLocal) cur.baseURL = baseURL.trim();
    providers[provider as string] = cur;
    await app.saveSettings();
  }

  async function test() {
    if (!provider) return;
    testing = true;
    result = null;
    try {
      await mergeAndSave();
      result = plain(await TestProvider(provider));
      if (provider === "claude") {
        // A successful check persists the resolved ClaudeBin server-side;
        // reload so providerConfigured() sees it.
        app.settings = plain(await GetSettings());
      }
    } finally {
      testing = false;
    }
  }

  async function finish() {
    if (!provider) return;
    await mergeAndSave();
    app.settings.defaultProvider = provider;
    app.settings.setupCompleted = true;
    await app.saveSettings();
    app.showBanner(copy.setup.ready(PROVIDER_NAMES[provider as ProviderId]));
    leave();
  }

  async function skip() {
    app.settings.setupCompleted = true;
    await app.saveSettings();
    leave();
  }

  function leave() {
    const to = app.setupReturn;
    app.setupStep = 1;
    app.setupProvider = "";
    app.setupReturn = "convert";
    app.go(to);
  }

  const HOWTO_FALLBACK = copy.ui.llamacppBlurb;

  function emptyProvider() {
    return { apiKey: undefined, baseURL: undefined, model: undefined };
  }

  function resultClass(r: KeycheckResult): string {
    if (r.status === "ok") return "ok";
    if (r.status === "no_credits" || r.status === "model_missing" || r.status === "not_logged_in")
      return "warn";
    return "danger";
  }
</script>

<div class="screen">
  <div class="wizard">
    <div class="wizard-top">
      <span class="wizard-step nums">{copy.ui.step(step, 3)}</span>
      <button class="text-btn" onclick={skip}>{copy.setup.skip}</button>
    </div>

    {#if step === 1}
      <h1>{copy.setup.welcomeTitle}</h1>
      <p class="wizard-lead">{copy.setup.welcomeBody}</p>
      <button class="btn primary big" onclick={() => (app.setupStep = 2)}>{copy.setup.getStarted}</button>
    {:else if step === 2}
      <h1>{copy.setup.pickTitle}</h1>
      <p class="wizard-lead">{copy.setup.pickLead}</p>
      <div class="provider-cards">
        {#each PROVIDER_CARDS as card (card.id)}
          <button class="provider-card" onclick={() => selectProvider(card.id)}>
            <span class="provider-name">
              {card.name}
              {#if card.badge}<span class="badge">{card.badge}</span>{/if}
            </span>
            <span class="provider-desc">{card.blurb}</span>
          </button>
          {#if card.id === "ollama"}
            <button class="link advanced-link" onclick={() => selectProvider("llamacpp")}>
              {copy.setup.llamacppLink}
            </button>
          {/if}
        {/each}
      </div>
    {:else if provider}
      <button class="link" onclick={() => (app.setupStep = 2)}>{copy.setup.back}</button>
      <h1>{PROVIDER_NAMES[provider as ProviderId]}</h1>
      <p class="wizard-lead">
        {PROVIDER_CARDS.find((c) => c.id === provider)?.blurb ?? HOWTO_FALLBACK}
      </p>

      <ProviderHowto provider={provider as ProviderId} />

      {#if needsKey}
        <KeyField bind:value={apiKey} label={copy.setup.keyLabel} placeholder={provider === "openrouter" ? "sk-or-…" : ""} />
      {/if}
      {#if isLocal}
        <label class="field">
          <span class="field-label">{copy.setup.baseUrlLabel}</span>
          <span class="field-input">
            <input
              type="text"
              bind:value={baseURL}
              placeholder={provider === "ollama"
                ? copy.setup.ollamaUrlPlaceholder
                : copy.setup.llamacppUrlPlaceholder}
              autocomplete="off"
              spellcheck="false"
            />
          </span>
        </label>
      {/if}

      <div class="row-actions">
        <button class="btn" disabled={testing || (needsKey && !apiKey.trim())} onclick={test}>
          {testing
            ? copy.setup.testing
            : provider === "claude"
              ? copy.setup.testClaude
              : copy.setup.test}
        </button>
        <button class="btn primary" disabled={!okTested} onclick={finish}>{copy.setup.finish}</button>
      </div>

      {#if result}
        <div class="test-result {resultClass(result)}">
          {result.detail || (result.status === "ok" ? copy.ui.connected : copy.ui.testFailed)}
          {#if result.warning}
            <div><span class="chip-warn">{result.warning}</span></div>
          {/if}
        </div>
      {/if}
    {/if}
  </div>
</div>
