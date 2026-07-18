const DATE_OPTIONS = {
  day: "2-digit",
  month: "2-digit",
  year: "numeric",
} as const;

const berlinDateFormatters: Record<string, Intl.DateTimeFormat> = {
  de: new Intl.DateTimeFormat("de", {
    ...DATE_OPTIONS,
    timeZone: "Europe/Berlin",
  }),
  en: new Intl.DateTimeFormat("en", {
    ...DATE_OPTIONS,
    timeZone: "Europe/Berlin",
  }),
  ru: new Intl.DateTimeFormat("ru", {
    ...DATE_OPTIONS,
    timeZone: "Europe/Berlin",
  }),
  sq: new Intl.DateTimeFormat("sq", {
    ...DATE_OPTIONS,
    timeZone: "Europe/Berlin",
  }),
};

const utcDateFormatters: Record<string, Intl.DateTimeFormat> = {
  de: new Intl.DateTimeFormat("de", { ...DATE_OPTIONS, timeZone: "UTC" }),
  en: new Intl.DateTimeFormat("en", { ...DATE_OPTIONS, timeZone: "UTC" }),
  ru: new Intl.DateTimeFormat("ru", { ...DATE_OPTIONS, timeZone: "UTC" }),
  sq: new Intl.DateTimeFormat("sq", { ...DATE_OPTIONS, timeZone: "UTC" }),
};

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
  return (formatters[locale] ?? formatters.de!).format(date);
}
