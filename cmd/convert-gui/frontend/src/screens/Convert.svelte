<script lang="ts">
  import {
    Enqueue,
    PickDirectory,
    Preflight,
    QuoteBook,
    SuggestOutputPath,
    type QuoteResult,
  } from "../lib/api";
  import { app, errMsg } from "../lib/app.svelte";
  import { copy, OPENROUTER_MODELS, providerName, type ProviderId } from "../lib/copy";
  import { ALL_LANGS, langName } from "../lib/lang";
  import { basename, joinPath, newConvertState, plain } from "../lib/types";
  import AppBar from "../components/AppBar.svelte";
  import Banner from "../components/Banner.svelte";
  import Dropzone from "../components/Dropzone.svelte";
  import EstimateCard from "../components/EstimateCard.svelte";
  import LangTargets from "../components/LangTargets.svelte";
  import ProviderPicker from "../components/ProviderPicker.svelte";

  const c = $derived(app.convert);

  let quotes = $state<Record<string, QuoteResult>>({});
  let preflightError = $state<{ text: string; setup: boolean } | null>(null);
  let converting = $state(false);

  const noLangWarning = $derived(
    !!c.estimate?.warnings?.some((w) => w.includes("no language metadata found")),
  );

  /** Source options: the known list plus whatever was detected. */
  const sourceOptions = $derived.by(() => {
    const opts = [...ALL_LANGS];
    if (c.source && !opts.includes(c.source)) opts.unshift(c.source);
    return opts;
  });

  // Re-quote every provider/model row when the inputs that price a book change.
  $effect(() => {
    const est = c.estimate;
    if (c.stage !== "configure" || !est) return;
    const src = c.source;
    const targets = $state.snapshot(c.targets) as string[];
    const repair = c.repair;
    if (targets.length === 0) return;
    const rows: [string, string][] = [
      ...OPENROUTER_MODELS.map((m) => ["openrouter", m.id] as [string, string]),
      ["gonka", ""],
      ["claude", ""],
      ["ollama", ""],
      ["llamacpp", ""],
    ];
    for (const [p, m] of rows) {
      QuoteBook(est.chars, est.sentences, src, targets, p, m, repair)
        .then((q) => {
          quotes[`${p}|${m}`] = plain(q);
        })
        .catch(() => {});
    }
  });

  // Keep the suggested output path in step with title/source/targets; a
  // user-chosen folder replaces the directory part but keeps Go's filename.
  $effect(() => {
    if (c.stage !== "configure" || !c.inputPath) return;
    const input = c.inputPath;
    const title = c.estimate?.title ?? "";
    const src = c.source;
    const targets = $state.snapshot(c.targets) as string[];
    const dir = c.outputDir;
    if (targets.length === 0) {
      c.outputPath = "";
      return;
    }
    SuggestOutputPath(input, title, src, targets)
      .then((suggested) => {
        c.outputPath = dir ? joinPath(dir, basename(suggested)) : suggested;
      })
      .catch(() => {});
  });

  async function changeDir() {
    try {
      const dir = await PickDirectory(copy.ui.chooseSaveDir);
      if (dir) c.outputDir = dir;
    } catch {
      // dialog dismissed
    }
  }

  const selectedQuote = $derived(quotes[`${c.provider}|${c.model}`]);
  const convertLabel = $derived.by(() => {
    if (converting) return copy.convert.checking;
    if (c.estimate && selectedQuote) return `${copy.convert.convert} — ${selectedQuote.display}`;
    return copy.convert.convert;
  });

  const canConvert = $derived(
    !converting && !!c.provider && c.targets.length > 0 && !!c.outputPath && (c.isTbook || !!c.source),
  );

  function openProviderSetup(p: ProviderId) {
    app.openSetup(3, p, "convert");
  }

  async function startConvert() {
    if (!canConvert) return;
    converting = true;
    preflightError = null;
    try {
      const res = plain(await Preflight(c.provider));
      if (res.status !== "ok") {
        preflightError =
          res.status === "invalid_key"
            ? { text: copy.convert.needsSetup(providerName(c.provider)), setup: true }
            : { text: res.detail || copy.convert.needsSetup(providerName(c.provider)), setup: res.status === "not_installed" };
        return;
      }
      const wasRunning = !!app.runningJob;
      const job = plain(
        await Enqueue({
          inputPath: c.inputPath,
          outputPath: c.outputPath,
          source: c.isTbook ? "" : c.source,
          targets: $state.snapshot(c.targets) as string[],
          provider: c.provider,
          model: c.provider === "openrouter" ? c.model : undefined,
          limitChapters: c.limitChapters || undefined,
          repair: c.repair,
          repairContext: c.repairContext || undefined,
          judge: c.judge || undefined,
          force: c.force || undefined,
          estimate: c.estimate ?? undefined,
        }),
      );
      app.upsertJob(job);
      app.convert = newConvertState();
      if (wasRunning) {
        app.showBanner(copy.convert.queuedBanner, "info");
        app.go("jobs");
      } else {
        app.openJob(job.id);
      }
    } catch (err) {
      preflightError = { text: errMsg(err), setup: false };
    } finally {
      converting = false;
    }
  }
</script>

<div class="screen">
  <AppBar title={copy.convert.title} />

  <div class="page">
    {#if c.stage === "pick"}
      <Dropzone onpick={(p) => app.openBook(p)} />
    {:else if c.stage === "estimating"}
      <div class="card" style="text-align:center; padding:48px 24px">
        <div class="progress-track big" style="max-width:320px; margin:0 auto 16px">
          <div class="progress-fill indeterminate"></div>
        </div>
        {copy.convert.estimating}
      </div>
    {:else}
      {#if c.isTbook}
        <Banner kind="info" text={copy.convert.tbookInfo} />
        <div class="card">
          <div class="card-title">{basename(c.inputPath)}</div>
          <div class="card-sub">{c.inputPath}</div>
        </div>
      {:else if c.estimate}
        {#if noLangWarning}
          <Banner kind="warn" text={copy.convert.noLangWarning} />
        {/if}
        <EstimateCard estimate={c.estimate} />
      {/if}

      <div class="setting-row">
        <span class="setting-label">{copy.convert.sourceLabel}</span>
        {#if c.isTbook}
          <select disabled>
            <option>{copy.convert.sourceFromBook}</option>
          </select>
        {:else}
          <select
            class:attention={noLangWarning}
            value={c.source}
            onchange={(e) => {
              c.source = e.currentTarget.value;
              c.targets = c.targets.filter((t) => t !== c.source);
            }}
          >
            {#if !c.source}
              <option value="">{copy.ui.choose}</option>
            {/if}
            {#each sourceOptions as code (code)}
              <option value={code}>{langName(code)}</option>
            {/each}
          </select>
        {/if}
      </div>

      <div class="setting-row" style="display:block">
        <span class="setting-label">
          {copy.convert.targetsLabel}
          <span class="setting-hint">{copy.convert.targetsHint}</span>
        </span>
        <div style="margin-top:10px">
          <LangTargets
            bind:selected={c.targets}
            source={c.source}
            defaults={app.settings.defaultTargets ?? []}
          />
        </div>
      </div>

      <div class="setting-section">{copy.convert.engineLabel}</div>
      <ProviderPicker
        bind:provider={c.provider}
        bind:model={c.model}
        {quotes}
        showPrices={!!c.estimate}
        configured={(n) => app.providerConfigured(n)}
        onsetup={openProviderSetup}
        onchange={() => (preflightError = null)}
      />

      <div class="setting-row">
        <span class="setting-label">{copy.convert.saveToLabel}</span>
        <span class="setting-value" title={c.outputPath}>{c.outputPath}</span>
        <button class="btn" onclick={changeDir}>{copy.convert.change}</button>
      </div>

      <details class="advanced">
        <summary>{copy.convert.advanced}</summary>
        <div class="setting-row">
          <span class="setting-label">
            {copy.convert.previewLabel}
            <span class="setting-hint">{copy.convert.previewHint}</span>
          </span>
          <div class="segmented">
            {#each [0, 1, 3, 5] as n (n)}
              <button class:active={c.limitChapters === n} onclick={() => (c.limitChapters = n)}>
                {n === 0 ? copy.convert.off : `${n}`}
              </button>
            {/each}
          </div>
        </div>
        <div class="setting-row">
          <span class="setting-label">
            {copy.convert.proofreadLabel}
            <span class="setting-hint">{copy.convert.proofreadHint}</span>
          </span>
          <div class="segmented">
            <button class:active={c.repair === null} onclick={() => (c.repair = null)}>{copy.convert.auto}</button>
            <button class:active={c.repair === true} onclick={() => (c.repair = true)}>{copy.convert.on}</button>
            <button class:active={c.repair === false} onclick={() => (c.repair = false)}>{copy.convert.off}</button>
          </div>
        </div>
        <div class="setting-row">
          <span class="setting-label">
            {copy.convert.contextLabel}
            <span class="setting-hint">{copy.convert.contextHint}</span>
          </span>
          <!-- --context accepts 0 or 2 only (1 is a CLI hard error) -->
          <div class="segmented">
            <button class:active={c.repairContext === 0} onclick={() => (c.repairContext = 0)}>{copy.convert.none}</button>
            <button class:active={c.repairContext === 2} onclick={() => (c.repairContext = 2)}>{copy.convert.twoSentences}</button>
          </div>
        </div>
        <div class="setting-row">
          <span class="setting-label">
            {copy.convert.judgeLabel}
            <span class="setting-hint">{copy.convert.judgeHint}</span>
          </span>
          <input type="checkbox" bind:checked={c.judge} />
        </div>
        <div class="setting-row">
          <span class="setting-label">
            {copy.convert.forceLabel}
            <span class="setting-hint">{copy.convert.forceHint}</span>
          </span>
          <input type="checkbox" bind:checked={c.force} />
        </div>
      </details>

      {#if preflightError}
        <Banner
          kind="danger"
          text={preflightError.text}
          actionLabel={preflightError.setup ? copy.convert.setUpNow : ""}
          onaction={() => openProviderSetup(c.provider as ProviderId)}
          ondismiss={() => (preflightError = null)}
        />
      {/if}

      <div class="row-actions">
        <button class="btn primary big" disabled={!canConvert} onclick={startConvert}>
          {convertLabel}
        </button>
      </div>
      <p class="reassure">{copy.convert.reassure}</p>
    {/if}
  </div>
</div>
