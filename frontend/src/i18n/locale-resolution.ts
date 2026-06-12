import { DEFAULT_LOCALE, normalizeLocale, type AppLocale } from "./locales";

// Pure locale-resolution helpers, kept free of next-intl/server and
// next/headers so they are unit-testable in isolation. request.ts wires these
// into the next-intl request config; everything that can be reasoned about
// without a request context lives here.

export type Messages = Record<string, unknown>;

/**
 * Deep-merges `override` onto `base`, recursing into plain objects so a locale
 * catalog that omits a nested key inherits the German fallback value for it
 * rather than dropping the whole branch. Arrays and primitives in `override`
 * replace the base value outright.
 */
export function mergeMessages(base: Messages, override: Messages): Messages {
  const out: Messages = { ...base };
  for (const [key, value] of Object.entries(override)) {
    const current = out[key];
    if (
      value &&
      typeof value === "object" &&
      !Array.isArray(value) &&
      current &&
      typeof current === "object" &&
      !Array.isArray(current)
    ) {
      out[key] = mergeMessages(current as Messages, value as Messages);
    } else {
      out[key] = value;
    }
  }
  return out;
}

type LanguageCandidate = {
  tag: string;
  q: number;
  index: number;
};

function parseAcceptLanguage(value: string): LanguageCandidate[] {
  return value
    .split(",")
    .map((part, index): LanguageCandidate | null => {
      const [rawTag, ...rawParams] = part.trim().split(";");
      const tag = rawTag?.trim();
      if (!tag) return null;

      const qParam = rawParams
        .map((param) => param.trim())
        .find((param) => param.toLowerCase().startsWith("q="));
      const q = qParam ? Number(qParam.slice(2).trim()) : 1;

      if (!Number.isFinite(q) || q <= 0 || q > 1) return null;
      return { tag, q, index };
    })
    .filter((candidate): candidate is LanguageCandidate => Boolean(candidate))
    .sort((a, b) => b.q - a.q || a.index - b.index);
}

/**
 * Picks a supported locale from an Accept-Language header. Candidates are
 * evaluated by q-value priority, with original header order as the tie-breaker.
 * A supported non-default locale wins; a `de*` candidate resolves to the
 * default so an explicit German preference is honoured rather than falling
 * through to a later non-German tag.
 */
export function localeFromAcceptLanguage(value: string | null): AppLocale {
  if (!value) return DEFAULT_LOCALE;
  for (const candidate of parseAcceptLanguage(value)) {
    const locale = normalizeLocale(candidate.tag);
    if (
      locale !== DEFAULT_LOCALE ||
      candidate.tag.toLowerCase().startsWith("de")
    ) {
      return locale;
    }
  }
  return DEFAULT_LOCALE;
}

/**
 * Resolves the effective locale for a request. Non-localized surfaces (the
 * German-only staff/operator portals, where the proxy never sets the localize
 * header) always get the default; localized ones prefer an explicit cookie,
 * then fall back to the browser's Accept-Language.
 */
export function resolveLocale(input: {
  localized: boolean;
  cookieValue: string | undefined;
  acceptLanguage: string | null;
}): AppLocale {
  if (!input.localized) return DEFAULT_LOCALE;
  if (input.cookieValue) return normalizeLocale(input.cookieValue);
  return localeFromAcceptLanguage(input.acceptLanguage);
}
