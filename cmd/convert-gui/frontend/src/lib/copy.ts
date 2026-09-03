// Every user-facing string in one place.

export const PROVIDERS = ["openrouter", "gonka", "claude", "ollama", "llamacpp"] as const;
export type ProviderId = (typeof PROVIDERS)[number];

export const PROVIDER_NAMES: Record<ProviderId, string> = {
  openrouter: "OpenRouter",
  gonka: "Gonka",
  claude: "Claude",
  ollama: "Ollama",
  llamacpp: "llama.cpp",
};

export function providerName(id: string): string {
  return PROVIDER_NAMES[id as ProviderId] ?? id;
}

/** Setup wizard step 2: plain-language provider cards. */
export const PROVIDER_CARDS: { id: ProviderId; name: string; blurb: string; badge?: string }[] = [
  {
    id: "openrouter",
    name: "OpenRouter",
    blurb: "Cloud — best quality, about $1.30 per book, 15–30 minutes.",
    badge: "Recommended",
  },
  {
    id: "gonka",
    name: "Gonka",
    blurb: "Cloud — nearly free (about 1¢ per book), extra proofreading pass, slower.",
  },
  {
    id: "claude",
    name: "Claude",
    blurb: "Use my Claude subscription — long books may pause when the usage window runs out, then continue from the same point.",
  },
  {
    id: "ollama",
    name: "Ollama",
    blurb: "Fully local and free — needs about 10 GB of RAM; a book takes hours.",
  },
];

export interface HowtoStep {
  text: string;
  linkLabel?: string;
  url?: string;
  code?: string;
}
export interface Howto {
  title: string;
  steps: HowtoStep[];
  note: string;
}

export const HOWTO: Record<ProviderId, Howto> = {
  openrouter: {
    title: "How to get an OpenRouter key",
    steps: [
      { text: "Sign up — Google or GitHub works:", linkLabel: "openrouter.ai", url: "https://openrouter.ai" },
      {
        text: "Open your keys page:",
        linkLabel: "openrouter.ai/settings/keys",
        url: "https://openrouter.ai/settings/keys",
      },
      { text: "Click Create Key — name it anything." },
      { text: "Copy the key immediately — it is shown once and starts with sk-or-." },
      {
        text: "Add credits — $5 covers 3–4 books:",
        linkLabel: "openrouter.ai/settings/credits",
        url: "https://openrouter.ai/settings/credits",
      },
      { text: "Paste the key here and press Test connection." },
    ],
    note: "Your book's text is sent to OpenRouter's model providers during conversion.",
  },
  gonka: {
    title: "How to get a Gonka key",
    steps: [
      { text: "Sign in to the dashboard:", linkLabel: "proxy.gonka.gg/dashboard", url: "https://proxy.gonka.gg/dashboard" },
      { text: "Create an API key and copy it." },
      { text: "Paste it here and press Test connection." },
    ],
    note: "Nearly free (about 1¢ per book). Adds an automatic proofreading pass; can be slower at busy times.",
  },
  claude: {
    title: "How to connect Claude",
    steps: [
      { text: "You need a paid Claude plan — the same account you use at claude.ai." },
      { text: "Install Claude Code:", linkLabel: "claude.com/claude-code", url: "https://claude.com/claude-code" },
      { text: "Open it once and sign in." },
      { text: "Click Check Claude Code below." },
    ],
    note: "Long books may pause when your plan's usage window runs out — nothing is lost; the conversion continues from the same point when you retry.",
  },
  ollama: {
    title: "How to set up Ollama",
    steps: [
      { text: "Install Ollama:", linkLabel: "ollama.com/download", url: "https://ollama.com/download" },
      { text: "Start it — look for the tray or menu-bar icon." },
      { text: "Press Test connection. The translation model download (about 8 GB) is one-time." },
    ],
    note: "Free, private, fully offline. A book takes hours.",
  },
  llamacpp: {
    title: "How to set up llama.cpp (advanced)",
    steps: [
      { text: "Install llama.cpp on this computer." },
      {
        text: "Run the server:",
        code: "llama-server -hf ggml-org/gemma-3-4b-it-GGUF -np 2 -c 8192 --jinja",
      },
      { text: "Press Test connection — the served model is adopted automatically." },
    ],
    note: "For users already running their own llama.cpp server.",
  },
};

/** OpenRouter model sub-rows on the Convert screen. */
export const OPENROUTER_MODELS: { id: string; label: string; desc: string }[] = [
  { id: "google/gemini-3.7-flash", label: "Gemini 3.7 Flash", desc: "Best quality — recommended" },
  { id: "google/gemini-3.1-flash-lite", label: "Gemini 3.1 Flash Lite", desc: "Slightly cheaper, nearly as good" },
];

export const copy = {
  appName: "tBook Converter",

  ui: {
    loading: "Loading…",
    back: "Back",
    queue: "Queue",
    settings: "Settings",
    convertingPill: (title: string, pct: number) => `Converting ${title} — ${pct}%`,
    step: (n: number, total: number) => `Step ${n} of ${total}`,
    connected: "Connected.",
    testFailed: "Something went wrong — try again.",
    untitled: "Untitled",
    by: "by",
    factLanguage: "Language",
    factChapters: "Chapters",
    factWords: "Words",
    factSentences: "Sentences",
    factUnknown: "Unknown",
    footnoteSentences: (n: string) => `+ ${n} sentences in footnotes`,
    starting: "Starting…",
    waiting: "waiting…",
    allLanguages: "All languages",
    done: "Done",
    choose: "Choose…",
    chooseSaveDir: "Choose where to save",
    chooseOutputDir: "Choose the output folder",
    llamacppBlurb: "For users already running their own llama.cpp server.",
    status: {
      running: "Running",
      queued: "Queued",
      done: "Done",
      failed: "Failed",
      canceled: "Canceled",
      interrupted: "Interrupted",
    } as Record<string, string>,
  },

  setup: {
    welcomeTitle: "Welcome to tBook Converter",
    welcomeBody:
      "Turn an EPUB or FB2 book into a .tbook — a tap-to-translate ebook you read offline in the TReader app. " +
      "Translation runs through an engine of your choice; setting one up takes about two minutes.",
    getStarted: "Get started",
    skip: "Skip for now",
    pickTitle: "Choose a translation engine",
    pickLead: "You can add or change engines later in Settings.",
    llamacppLink: "Using llama.cpp? Advanced setup →",
    back: "← All engines",
    test: "Test connection",
    testClaude: "Check Claude Code",
    testing: "Testing…",
    finish: "Finish",
    keyLabel: "API key",
    baseUrlLabel: "Server address",
    ollamaUrlPlaceholder: "http://127.0.0.1:11434 (default)",
    llamacppUrlPlaceholder: "http://127.0.0.1:8080 (default)",
    ready: (p: string) => `${p} is ready — you can convert your first book.`,
  },

  convert: {
    title: "Convert a book",
    dropTitle: "Drop your book here or browse files",
    dropSub: ".epub, .fb2, .fb2.zip — or a .tbook to add another language",
    estimating: "Reading your book…",
    estimateFailed: "Couldn't read this book",
    tbookInfo:
      "This is already a .tbook — pick the languages to add. Existing translations are kept and reused.",
    noLangWarning: "This book doesn't say what language it's in — pick the original language below.",
    sourceLabel: "Original language",
    sourceFromBook: "Kept from the book",
    targetsLabel: "Translate into",
    targetsHint: "Each language is a separate full translation — price and time multiply.",
    more: "More…",
    engineLabel: "Translation engine",
    setUp: "Set up",
    saveToLabel: "Save to",
    change: "Change…",
    advanced: "Advanced",
    previewLabel: "Preview first",
    previewHint: "Convert only the first chapters to check the quality cheaply.",
    proofreadLabel: "Proofreading",
    proofreadHint: "Auto: on for Gonka; elsewhere roughly doubles cost and time.",
    contextLabel: "Proofreading context",
    contextHint: "Give the proofreader the surrounding sentences.",
    judgeLabel: "Quality review",
    judgeHint: "An extra pass that flags doubtful sentences.",
    forceLabel: "Start from scratch",
    forceHint: "Ignore saved progress and re-translate everything.",
    convert: "Convert",
    checking: "Checking…",
    reassure: "Conversions continue in the background — you can keep using the app.",
    needsSetup: (p: string) => `${p} needs a one-time setup — it takes about two minutes.`,
    setUpNow: "Set up now",
    queuedBanner: "Added to the queue — it starts when the current conversion finishes.",
    off: "Off",
    none: "None",
    twoSentences: "2 sentences",
    auto: "Auto",
    on: "On",
    priceUnknown: "—",
  },

  job: {
    title: "Conversion",
    queuedTitle: "Waiting in the queue",
    queuedBody: "Starts automatically when the current conversion finishes.",
    startNow: "Start now",
    remove: "Remove",
    cancel: "Cancel conversion",
    cancelSheetTitle: "Cancel this conversion?",
    cancelSheetBody:
      "Everything translated so far is saved. You can resume later from exactly this point — nothing will be re-billed or re-translated.",
    cancelConfirm: "Cancel conversion",
    cancelKeep: "Keep converting",
    runningReassure: "You can close this window — progress is saved continuously.",
    spentSoFar: "Spent so far:",
    underCent: "under $0.01",
    claudeCost: "Using your Claude subscription",
    localCost: "Free — running on this computer",
    elapsed: "Elapsed",
    doneTitle: "Your book is ready",
    showInFolder: "Show in folder",
    convertAnother: "Convert another book",
    readerHint: "To read it: open the TReader app and import this .tbook.",
    totalCost: "Total cost:",
    failedTitle: "Something went wrong",
    usageLimit:
      "Your Claude plan's usage window is used up. Nothing is lost — retry after it resets and the conversion continues from exactly this point.",
    retry: "Retry — continues from where it stopped",
    startOver: "Start over",
    retryHint: "Retrying never re-translates finished sentences.",
    techDetails: "Technical details",
    canceledTitle: "Canceled",
    canceledBody: "Everything translated so far is saved — resume any time from exactly this point.",
    interruptedTitle: "Interrupted",
    interruptedBody:
      "The app was closed while this conversion was running. Nothing is lost — resume from exactly where it stopped.",
    resume: "Resume",
    notFound: "This conversion is no longer in the list.",
  },

  jobs: {
    title: "Queue",
    running: "Running",
    queued: "Queued",
    history: "History",
    empty: "No conversions yet — convert your first book from the home screen.",
    addLanguage: "Add another language",
    convertAgain: "Convert again",
    removeFromList: "Remove from list",
    removeSheetTitle: "Remove this conversion from the list?",
    removeSheetBody: "Only the list entry is removed. The .tbook file itself is not deleted.",
    removeConfirm: "Remove",
    removeKeep: "Keep",
  },

  settings: {
    title: "Settings",
    engines: "Translation engines",
    enginesHint: "Keys are stored on this computer only and sent only to their own provider.",
    keySaved: "Key saved",
    notSetUp: "Not set up",
    claudeFound: "Claude Code found",
    configured: "Configured",
    test: "Test",
    edit: "Edit",
    defaultEngine: "Default engine",
    addEngine: "Set up a translation engine…",
    defaults: "Defaults",
    defaultTargets: "Default target languages",
    outputDir: "Output folder",
    theme: "Theme",
    themeSystem: "System",
    themeLight: "Light",
    themeDark: "Dark",
    aligner: "Word alignment",
    alignerInstalled: "High-quality aligner: Installed",
    alignerMissing:
      "Not installed — using the AI model for alignment (works, but slower and slightly less precise).",
    alignerWindows: "Coming to Windows — conversions use the built-in aligner meanwhile.",
    alignerSoon: "Guided install is coming in a future update.",
    advanced: "Advanced",
    cacheDir: "Cache folder",
    cacheHint: "Saved translation progress lives here; retries and added languages reuse it.",
    defaultLocation: "Default location",
    about: "About",
    aboutBlurb: "tBook Converter — turn EPUB/FB2 books into tap-to-translate .tbook files.",
    website: "tbook.dev",
    websiteUrl: "https://tbook.dev",
    save: "Save",
    done: "Done",
  },
};
