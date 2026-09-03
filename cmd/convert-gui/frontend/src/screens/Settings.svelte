<script lang="ts">
  import { GetSettings, OpenExternal, PickDirectory, TestProvider, type KeycheckResult } from "../lib/api";
  import { app } from "../lib/app.svelte";
  import { copy, PROVIDER_NAMES, PROVIDERS, type ProviderId } from "../lib/copy";
  import { plain } from "../lib/types";
  import AppBar from "../components/AppBar.svelte";
  import KeyField from "../components/KeyField.svelte";
  import LangTargets from "../components/LangTargets.svelte";
  import ProviderHowto from "../components/ProviderHowto.svelte";

  const s = $derived(app.settings);

  // Inline test results per provider row.
  let rowResults = $state<Record<string, KeycheckResult>>({});
  let rowTesting = $state<Record<string, boolean>>({});

  // Edit sheet.
  let editProvider = $state<ProviderId | null>(null);
  let editKey = $state("");
  let editBaseURL = $state("");

  const THEMES = [
    { value: "system", label: copy.settings.themeSystem },
    { value: "light", label: copy.settings.themeLight },
    { value: "dark", label: copy.settings.themeDark },
  ];

  const configuredProviders = $derived(PROVIDERS.filter((p) => app.providerConfigured(p)));

  function providerStatus(p: ProviderId): string {
    if (p === "claude") return s.claudeBin ? copy.settings.claudeFound : copy.settings.notSetUp;
    if (p === "openrouter" || p === "gonka")
      return s.providers?.[p]?.apiKey ? copy.settings.keySaved : copy.settings.notSetUp;
    return p in (s.providers ?? {}) ? copy.settings.configured : copy.settings.notSetUp;
  }

  async function testRow(p: ProviderId) {
    rowTesting[p] = true;
    delete rowResults[p];
    try {
      rowResults[p] = plain(await TestProvider(p));
      if (p === "claude") app.settings = plain(await GetSettings());
    } finally {
      rowTesting[p] = false;
    }
  }

  function resultClass(r: KeycheckResult): string {
    if (r.status === "ok") return "ok";
    if (r.status === "no_credits" || r.status === "model_missing" || r.status === "not_logged_in")
      return "warn";
    return "danger";
  }

  function openEdit(p: ProviderId) {
    editProvider = p;
    editKey = s.providers?.[p]?.apiKey ?? "";
    editBaseURL = s.providers?.[p]?.baseURL ?? "";
  }

  async function saveEdit() {
    const p = editProvider;
    if (!p) return;
    const providers = app.settings.providers ?? (app.settings.providers = {});
    const cur = providers[p] ?? { apiKey: undefined, baseURL: undefined, model: undefined };
    if (p === "openrouter" || p === "gonka" || p === "llamacpp") cur.apiKey = editKey.trim();
    if (p === "ollama" || p === "llamacpp") cur.baseURL = editBaseURL.trim();
    providers[p] = cur;
    await app.saveSettings();
    editProvider = null;
  }

  async function save() {
    await app.saveSettings();
  }

  async function changeOutputDir() {
    try {
      const dir = await PickDirectory(copy.ui.chooseOutputDir);
      if (dir) {
        app.settings.outputDir = dir;
        await save();
      }
    } catch {
      // dialog dismissed
    }
  }

  function setTheme(t: string) {
    app.settings.ui = { theme: t };
    save();
  }
</script>

<div class="screen">
  <AppBar title={copy.settings.title} back="convert" />

  <div class="page">
    <div class="setting-section">{copy.settings.engines}</div>
    <p class="setting-hint">{copy.settings.enginesHint}</p>
    {#each PROVIDERS as p (p)}
      <div class="setting-row">
        <span class="setting-label">
          {PROVIDER_NAMES[p]}
          <span class="setting-hint">{providerStatus(p)}</span>
          {#if rowResults[p]}
            <span class="test-result {resultClass(rowResults[p])}" style="display:inline-block; margin-top:6px">
              {rowResults[p].detail || (rowResults[p].status === "ok" ? copy.ui.connected : "")}
              {#if rowResults[p].warning}
                <span class="chip-warn">{rowResults[p].warning}</span>
              {/if}
            </span>
          {/if}
        </span>
        <span style="display:flex; gap:8px; flex:none">
          <button class="btn" disabled={!!rowTesting[p]} onclick={() => testRow(p)}>
            {rowTesting[p] ? copy.setup.testing : copy.settings.test}
          </button>
          {#if p !== "claude"}
            <button class="btn" onclick={() => openEdit(p)}>{copy.settings.edit}</button>
          {/if}
        </span>
      </div>
    {/each}

    <div class="setting-row">
      <span class="setting-label">{copy.settings.defaultEngine}</span>
      <select
        value={s.defaultProvider}
        onchange={(e) => {
          app.settings.defaultProvider = e.currentTarget.value;
          save();
        }}
      >
        {#each configuredProviders as p (p)}
          <option value={p}>{PROVIDER_NAMES[p]}</option>
        {/each}
      </select>
    </div>
    <div class="setting-row">
      <button class="link" onclick={() => app.openSetup(2, "", "settings")}>
        {copy.settings.addEngine}
      </button>
    </div>

    <div class="setting-section">{copy.settings.defaults}</div>
    <div class="setting-row" style="display:block">
      <span class="setting-label">{copy.settings.defaultTargets}</span>
      <div style="margin-top:10px">
        <LangTargets bind:selected={app.settings.defaultTargets} onchange={save} />
      </div>
    </div>
    <div class="setting-row">
      <span class="setting-label">{copy.settings.outputDir}</span>
      <span class="setting-value">{s.outputDir || app.info.defaultOutputDir}</span>
      <button class="btn" onclick={changeOutputDir}>{copy.convert.change}</button>
    </div>
    <div class="setting-row">
      <span class="setting-label">{copy.settings.theme}</span>
      <div class="segmented">
        {#each THEMES as t (t.value)}
          <button
            class:active={(s.ui?.theme || "system") === t.value}
            onclick={() => setTheme(t.value)}
          >
            {t.label}
          </button>
        {/each}
      </div>
    </div>

    <div class="setting-section">{copy.settings.aligner}</div>
    <div class="setting-row">
      <span class="setting-label">
        {app.info.alignerInstalled ? copy.settings.alignerInstalled : copy.settings.alignerMissing}
        {#if !app.info.alignerInstalled}
          <span class="setting-hint">
            {app.info.platform === "windows" ? copy.settings.alignerWindows : copy.settings.alignerSoon}
          </span>
        {/if}
      </span>
    </div>

    <div class="setting-section">{copy.settings.advanced}</div>
    <div class="setting-row">
      <span class="setting-label">
        {copy.convert.proofreadLabel}
        <span class="setting-hint">{copy.convert.proofreadHint}</span>
      </span>
      <div class="segmented">
        <button
          class:active={s.repair == null}
          onclick={() => {
            app.settings.repair = null;
            save();
          }}>{copy.convert.auto}</button
        >
        <button
          class:active={s.repair === true}
          onclick={() => {
            app.settings.repair = true;
            save();
          }}>{copy.convert.on}</button
        >
        <button
          class:active={s.repair === false}
          onclick={() => {
            app.settings.repair = false;
            save();
          }}>{copy.convert.off}</button
        >
      </div>
    </div>
    <div class="setting-row">
      <span class="setting-label">{copy.convert.contextLabel}</span>
      <div class="segmented">
        <button
          class:active={(s.repairContext ?? 0) === 0}
          onclick={() => {
            app.settings.repairContext = 0;
            save();
          }}>{copy.convert.none}</button
        >
        <button
          class:active={s.repairContext === 2}
          onclick={() => {
            app.settings.repairContext = 2;
            save();
          }}>{copy.convert.twoSentences}</button
        >
      </div>
    </div>
    <div class="setting-row">
      <span class="setting-label">
        {copy.convert.judgeLabel}
        <span class="setting-hint">{copy.convert.judgeHint}</span>
      </span>
      <input
        type="checkbox"
        checked={s.judge ?? false}
        onchange={(e) => {
          app.settings.judge = e.currentTarget.checked;
          save();
        }}
      />
    </div>
    <div class="setting-row">
      <span class="setting-label">
        {copy.settings.cacheDir}
        <span class="setting-hint">{copy.settings.cacheHint}</span>
      </span>
      <span class="setting-value">{s.cacheDir || copy.settings.defaultLocation}</span>
    </div>

    <div class="setting-section">{copy.settings.about}</div>
    <div class="about">
      <p><strong>{copy.appName}</strong> — {copy.settings.aboutBlurb}</p>
      <p class="nums">
        Version {app.info.version || "dev"}{app.info.commit ? ` (${app.info.commit.slice(0, 8)})` : ""}
        · {app.info.platform}
      </p>
      <p>
        <button class="link" onclick={() => OpenExternal(copy.settings.websiteUrl)}>
          {copy.settings.website}
        </button>
      </p>
    </div>
  </div>
</div>

{#if editProvider}
  <div class="sheet-backdrop">
    <div class="sheet" role="dialog" aria-label="Edit engine">
      <div class="sheet-handle"></div>
      <div class="sheet-title">{PROVIDER_NAMES[editProvider]}</div>
      <ProviderHowto provider={editProvider} />
      {#if editProvider === "openrouter" || editProvider === "gonka" || editProvider === "llamacpp"}
        <KeyField bind:value={editKey} label={copy.setup.keyLabel} />
      {/if}
      {#if editProvider === "ollama" || editProvider === "llamacpp"}
        <label class="field">
          <span class="field-label">{copy.setup.baseUrlLabel}</span>
          <span class="field-input">
            <input
              type="text"
              bind:value={editBaseURL}
              placeholder={editProvider === "ollama"
                ? copy.setup.ollamaUrlPlaceholder
                : copy.setup.llamacppUrlPlaceholder}
              autocomplete="off"
              spellcheck="false"
            />
          </span>
        </label>
      {/if}
      <div class="sheet-actions">
        <button class="btn" onclick={() => (editProvider = null)}>{copy.settings.done}</button>
        <button class="btn primary" onclick={saveEdit}>{copy.settings.save}</button>
      </div>
    </div>
  </div>
{/if}
