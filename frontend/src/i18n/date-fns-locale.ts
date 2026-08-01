import type { Locale } from "date-fns";
import { de, enUS, ru, sq } from "date-fns/locale";

import { normalizeLocale, type AppLocale } from "./locales";

// The kit calendar renders month and weekday names through date-fns, so the
// app's locale codes need a date-fns counterpart. Keyed by AppLocale, so adding
// a language to locales.generated.ts fails the type check here until the
// calendar can speak it too.
const DATE_FNS_LOCALES: Record<AppLocale, Locale> = {
  de,
  en: enUS,
  ru,
  sq,
};

/**
 * date-fns locale for a next-intl locale code. Unknown input falls back to the
 * default locale, matching `normalizeLocale`.
 */
export function dateFnsLocaleFor(locale: string | null | undefined): Locale {
  return DATE_FNS_LOCALES[normalizeLocale(locale)];
}
