import {
  normalizeLocale,
  SUPPORTED_LOCALES,
  type AppLocale,
} from "~/i18n/locales";

const DATE_OPTIONS = {
  day: "2-digit",
  month: "2-digit",
  year: "numeric",
} as const;

const DATE_TIME_OPTIONS = {
  ...DATE_OPTIONS,
  hour: "2-digit",
  minute: "2-digit",
} as const;

// One formatter per registered locale, built from the shared locale list.
// Adding a language to locales.json therefore gives it its own date format
// automatically instead of silently rendering German dates.
function formattersFor(
  options: Intl.DateTimeFormatOptions,
): Record<AppLocale, Intl.DateTimeFormat> {
  return Object.fromEntries(
    SUPPORTED_LOCALES.map(({ code }) => [
      code,
      new Intl.DateTimeFormat(code, options),
    ]),
  ) as Record<AppLocale, Intl.DateTimeFormat>;
}

const berlinDateFormatters = formattersFor({
  ...DATE_OPTIONS,
  timeZone: "Europe/Berlin",
});

const utcDateFormatters = formattersFor({ ...DATE_OPTIONS, timeZone: "UTC" });

const berlinDateTimeFormatters = formattersFor({
  ...DATE_TIME_OPTIONS,
  timeZone: "Europe/Berlin",
});

/**
 * Format a DATE-column value without converting it through the viewer's
 * timezone. UTC is used only as a neutral container for the calendar fields;
 * the input does not represent an instant.
 */
export function formatCalendarDate(
  iso: string,
  locale: string,
  options: Intl.DateTimeFormatOptions = DATE_OPTIONS,
): string {
  const date = new Date(`${iso}T00:00:00Z`);
  if (Number.isNaN(date.getTime())) return iso;
  return new Intl.DateTimeFormat(locale, {
    ...options,
    timeZone: "UTC",
  }).format(date);
}

export function formatLocalizedDate(iso: string, locale: string): string {
  const isDateOnly = iso.length === 10;
  const date = new Date(isDateOnly ? `${iso}T00:00:00Z` : iso);
  if (Number.isNaN(date.getTime())) return iso;
  const formatters = isDateOnly ? utcDateFormatters : berlinDateFormatters;
  return formatters[normalizeLocale(locale)].format(date);
}

/**
 * Format an instant (RFC3339 timestamp) as a localized date AND time, pinned to
 * the school's Europe/Berlin wall clock so a deadline like "09:00" reads the
 * same for every viewer regardless of their browser timezone. Use for real
 * instants such as enrollment_close_at — never for DATE-only calendar values,
 * which have no meaningful time-of-day (use formatLocalizedDate for those).
 */
export function formatLocalizedDateTime(iso: string, locale: string): string {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return iso;
  return berlinDateTimeFormatters[normalizeLocale(locale)].format(date);
}
