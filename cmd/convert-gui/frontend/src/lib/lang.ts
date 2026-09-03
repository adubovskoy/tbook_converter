// Port of desktop/src/lib/lang.ts (itself a port of android ui/Lang.kt).

export const LANG_NAMES: Record<string, string> = {
  en: "English",
  ru: "Russian",
  de: "German",
  es: "Spanish",
  fr: "French",
  it: "Italian",
  pt: "Portuguese",
  uk: "Ukrainian",
  pl: "Polish",
  ja: "Japanese",
  zh: "Chinese",
  ko: "Korean",
  tr: "Turkish",
  nl: "Dutch",
  ar: "Arabic",
};

export function langName(code: string): string {
  return LANG_NAMES[code] ?? code.toUpperCase();
}

/** Every language the "More…" sheet offers. */
export const ALL_LANGS = Object.keys(LANG_NAMES);

/** Quick target chips shown inline on the Convert screen (source excluded). */
export const QUICK_TARGETS = ["ru", "de", "es", "fr", "it", "pt", "pl", "uk", "zh", "ja", "en"];
