/** Weekday codes as the backend sends them (`mon` … `sun`). */
export const WEEKDAY_CODES = [
  "mon",
  "tue",
  "wed",
  "thu",
  "fri",
  "sat",
  "sun",
] as const;

export type WeekdayCode = (typeof WEEKDAY_CODES)[number];

/** The five school days, the default care week. */
export const SCHOOL_WEEKDAY_CODES = WEEKDAY_CODES.slice(0, 5);

const SHORT_LABELS: Readonly<Record<WeekdayCode, string>> = {
  mon: "Mo",
  tue: "Di",
  wed: "Mi",
  thu: "Do",
  fri: "Fr",
  sat: "Sa",
  sun: "So",
};

/** „Mo“ … „So“; an unknown code is shown as sent. */
export function weekdayShortLabel(code: string): string {
  return SHORT_LABELS[code as WeekdayCode] ?? code;
}

/** ISO weekday number (Monday = 1), or undefined for an unknown code. */
export function weekdayISONumber(code: string): number | undefined {
  const index = WEEKDAY_CODES.indexOf(code as WeekdayCode);
  return index < 0 ? undefined : index + 1;
}

/** „Mo“ … „So“ for an ISO weekday number (Monday = 1). */
export function isoWeekdayShortLabel(isoWeekday: number): string | undefined {
  const code = WEEKDAY_CODES[isoWeekday - 1];
  return code ? SHORT_LABELS[code] : undefined;
}

export function isSchoolWeekday(code: string): boolean {
  return (SCHOOL_WEEKDAY_CODES as readonly string[]).includes(code);
}
