/**
 * Die Rechnung der Seite „Stichtagszahlen" (#4673): welcher Monat gewählt ist,
 * welche Zahlen jede Schule in diesem Monat hat und wie sie sich zum Monat
 * davor verändert haben. Reine Funktionen, damit Seite und Test dieselbe
 * Rechnung lesen.
 */
import { berlinTodayISO, parseISODate } from "~/lib/date-helpers";
import type {
  TraegerKeyDateCount,
  TraegerKeyDateMonth,
  TraegerSchoolKeyDateCounts,
} from "~/lib/traeger-api";

const monthFormat = new Intl.DateTimeFormat("de-DE", {
  month: "long",
  year: "numeric",
});

/** „Oktober 2026" für den ersten Tag eines Monats (`YYYY-MM-DD`). */
export function formatKeyDateMonth(period: string): string {
  return monthFormat.format(parseISODate(period));
}

/** Ein Monat mit allen tatsächlich erfassten Stichtagen seiner Schulen. */
export interface KeyDateMonthRow extends TraegerKeyDateMonth {
  readonly keyDates: readonly string[];
}

export function keyDateMonthRows(
  schools: readonly TraegerSchoolKeyDateCounts[],
  months: readonly TraegerKeyDateMonth[],
): KeyDateMonthRow[] {
  const dates = new Map<string, Set<string>>();
  for (const school of schools) {
    for (const count of school.counts) {
      let days = dates.get(count.period);
      if (!days) {
        days = new Set<string>();
        dates.set(count.period, days);
      }
      days.add(count.keyDate);
    }
  }
  return months.map((month) => ({
    ...month,
    keyDates: [...(dates.get(month.period) ?? [])].sort(),
  }));
}

/** Eine Schule im gewählten Monat und im erfassten Monat davor. */
export interface KeyDateSchoolRow {
  readonly school: TraegerSchoolKeyDateCounts;
  /** `null`: Für diese Schule gibt es zu diesem Stichtag keine Zahlen. */
  readonly count: TraegerKeyDateCount | null;
  readonly previous: TraegerKeyDateCount | null;
}

/** Der gewählte Monat mit seinem Vormonat und den Zeilen je Schule. */
export interface KeyDateSelection {
  readonly month: TraegerKeyDateMonth;
  /** Der nächstältere erfasste Monat; `null` beim ersten erfassten Monat. */
  readonly previous: TraegerKeyDateMonth | null;
  readonly rows: readonly KeyDateSchoolRow[];
}

/**
 * Der Monat `period` mit seinem Vormonat, oder der neueste Monat, wenn
 * `period` fehlt oder unbekannt ist. `null` ohne erfassten Monat.
 */
export function selectKeyDateMonth(
  schools: readonly TraegerSchoolKeyDateCounts[],
  months: readonly TraegerKeyDateMonth[],
  period: string | null,
): KeyDateSelection | null {
  const found = months.findIndex((month) => month.period === period);
  const index = found >= 0 ? found : 0;
  const month = months[index];
  if (!month) return null;
  const previous = months[index + 1] ?? null;
  return {
    month,
    previous,
    rows: schools.map((school) => ({
      school,
      count: countOf(school, month.period),
      previous: previous ? countOf(school, previous.period) : null,
    })),
  };
}

function countOf(
  school: TraegerSchoolKeyDateCounts,
  period: string,
): TraegerKeyDateCount | null {
  return school.counts.find((count) => count.period === period) ?? null;
}

/** Die Veränderung zum Vormonat; `null`, wenn eine der Zahlen fehlt. */
export function keyDateChange(
  current: number | undefined,
  previous: number | undefined,
): number | null {
  if (current === undefined || previous === undefined) return null;
  return current - previous;
}

/** „+3", „−2" (echtes Minuszeichen) oder „±0". */
export function formatKeyDateChange(change: number): string {
  if (change > 0) return `+${change}`;
  if (change < 0) return `−${Math.abs(change)}`;
  return "±0";
}

/**
 * Ob moto die Zahlen erst nach dem Stichtag festgehalten hat: dann lief die
 * Erfassung am Stichtag nicht und wurde im selben Monat nachgeholt.
 */
export function capturedLate(count: TraegerKeyDateCount): boolean {
  const recorded = berlinDay(count.recordedAt);
  return recorded !== null && recorded > count.keyDate;
}

function berlinDay(iso: string): string | null {
  const date = new Date(iso);
  return Number.isNaN(date.getTime()) ? null : berlinTodayISO(date);
}
