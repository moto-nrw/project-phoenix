/**
 * Display helpers of the monthly contribution (#4684): month names, weekday
 * labels and the price matrix of one school at a month. Calendar months stay
 * "YYYY-MM-01" strings; no Date crosses a time zone here.
 */

import type {
  CareModule,
  CareMonthRule,
  CareRunStatus,
  CareTariffs,
  CareWeekday,
} from "~/lib/traeger-care-billing-api";
import { CARE_WEEKDAYS } from "~/lib/traeger-care-billing-api";

export const WEEKDAY_SHORT: Record<CareWeekday, string> = {
  mon: "Mo",
  tue: "Di",
  wed: "Mi",
  thu: "Do",
  fri: "Fr",
  sat: "Sa",
  sun: "So",
};

export const WEEKDAY_LONG: Record<CareWeekday, string> = {
  mon: "Montag",
  tue: "Dienstag",
  wed: "Mittwoch",
  thu: "Donnerstag",
  fri: "Freitag",
  sat: "Samstag",
  sun: "Sonntag",
};

const MONTH_NAMES = [
  "Januar",
  "Februar",
  "März",
  "April",
  "Mai",
  "Juni",
  "Juli",
  "August",
  "September",
  "Oktober",
  "November",
  "Dezember",
] as const;

export const CARE_RUN_STATUS_LABELS: Record<CareRunStatus, string> = {
  calculated: "Vorschau",
  released: "Freigegeben",
};

export const CARE_RUN_STATUS_TONES: Record<CareRunStatus, "orange" | "green"> =
  {
    calculated: "orange",
    released: "green",
  };

export const MONTH_RULE_LABELS: Record<CareMonthRule, string> = {
  version_on_first: "Voller Monat nach dem Vertrag am 1.",
  pro_rata: "Anteilig nach Kalendertagen",
};

/** "2027-01-01" → { year: 2027, month: 1 }; null for anything else. */
function monthParts(iso: string): { year: number; month: number } | null {
  const match = /^(\d{4})-(\d{2})-\d{2}$/.exec(iso);
  if (!match) return null;
  const month = Number(match[2]);
  if (month < 1 || month > 12) return null;
  return { year: Number(match[1]), month };
}

/** "2027-01-01" → "Januar 2027". */
export function formatMonth(iso: string): string {
  const parts = monthParts(iso);
  if (!parts) return iso;
  return `${MONTH_NAMES[parts.month - 1]} ${parts.year}`;
}

/** The first day of the month of a calendar day: "2026-11-07" → "2026-11-01". */
export function monthOf(iso: string): string {
  return `${iso.slice(0, 7)}-01`;
}

/** Moves a month by n months: ("2026-12-01", 1) → "2027-01-01". */
export function addMonths(iso: string, n: number): string {
  const parts = monthParts(iso);
  if (!parts) return iso;
  const index = parts.year * 12 + parts.month - 1 + n;
  const year = Math.floor(index / 12);
  const month = (index % 12) + 1;
  return `${year}-${String(month).padStart(2, "0")}-01`;
}

/** count months from `from` on, oldest first. */
export function monthRange(from: string, count: number): string[] {
  return Array.from({ length: count }, (_, i) => addMonths(from, i));
}

/** Every month from `from` to `to`, both included; empty when to < from. */
export function monthsFromTo(from: string, to: string): string[] {
  const start = monthParts(from);
  const end = monthParts(to);
  if (!start || !end) return [];
  const count = (end.year - start.year) * 12 + end.month - start.month + 1;
  return count > 0 ? monthRange(from, count) : [];
}

/** "Mo, Di, Do" for the booked weekdays in their order. */
export function weekdayList(weekdays: readonly CareWeekday[]): string {
  return CARE_WEEKDAYS.filter((day) => weekdays.includes(day))
    .map((day) => WEEKDAY_SHORT[day])
    .join(", ");
}

/** "14:00" → "bis 14:00 Uhr"; empty without an end time. */
export function endsAtLabel(endsAt: string): string {
  return endsAt ? `bis ${endsAt.slice(0, 5)} Uhr` : "";
}

export interface CareSchoolOption {
  readonly id: string;
  readonly name: string;
}

/**
 * The schools of the price list: those with booked or priced modules first,
 * then the others, each by name.
 */
export function careSchools(data: CareTariffs): CareSchoolOption[] {
  const withModules = new Set(data.modules.map((module) => module.schoolId));
  return data.schools
    .map((school) => ({ id: school.id, name: school.name }))
    .sort(
      (a, b) =>
        Number(withModules.has(b.id)) - Number(withModules.has(a.id)) ||
        a.name.localeCompare(b.name, "de"),
    );
}

/** The months a school has a price list from, oldest first. */
export function priceStands(data: CareTariffs, schoolId: string): string[] {
  return [
    ...new Set(
      data.tariffs
        .filter((tariff) => tariff.schoolId === schoolId)
        .map((tariff) => tariff.validFrom),
    ),
  ].sort();
}

/**
 * The price list that holds at `month`: the latest one starting on or
 * before it, else the earliest one; empty without any.
 */
export function standAt(stands: readonly string[], month: string): string {
  const started = stands.filter((stand) => stand <= month);
  return started.at(-1) ?? stands[0] ?? "";
}

/** One module of a school with its price per weekday at a month. */
export interface PriceRow {
  readonly careOfferingId: string;
  readonly moduleName: string;
  readonly endsAt: string;
  readonly contractCount: number;
  readonly bookedWeekdays: readonly CareWeekday[];
  /** Monthly price in cent per weekday; missing when none holds. */
  readonly prices: Partial<Record<CareWeekday, number>>;
}

/** The modules of a school with the price of every weekday at `month`. */
export function priceRowsAt(
  data: CareTariffs,
  schoolId: string,
  month: string,
): PriceRow[] {
  const modules = data.modules.filter((module) => module.schoolId === schoolId);
  return modules.map((module: CareModule) => {
    const prices: Partial<Record<CareWeekday, number>> = {};
    for (const tariff of data.tariffs) {
      if (
        tariff.schoolId !== schoolId ||
        tariff.careOfferingId !== module.careOfferingId ||
        tariff.validFrom > month ||
        (tariff.validUntil !== null && tariff.validUntil < month)
      ) {
        continue;
      }
      prices[tariff.weekday] = tariff.monthlyCents;
    }
    return {
      careOfferingId: module.careOfferingId,
      moduleName: module.moduleName,
      endsAt: module.endsAt,
      contractCount: module.contractCount,
      bookedWeekdays: module.bookedWeekdays,
      prices,
    };
  });
}

/** Monday to Friday, and the weekend days a row books or prices. */
export function shownWeekdays(rows: readonly PriceRow[]): CareWeekday[] {
  return CARE_WEEKDAYS.filter(
    (day) =>
      (day !== "sat" && day !== "sun") ||
      rows.some(
        (row) =>
          row.bookedWeekdays.includes(day) || row.prices[day] !== undefined,
      ),
  );
}

/** The weekdays a row books that have no price at the month. */
export function missingPrices(row: PriceRow): CareWeekday[] {
  return row.bookedWeekdays.filter((day) => row.prices[day] === undefined);
}

/** "1 Vertrag" / "12 Verträge". */
export function contractCountLabel(count: number): string {
  return count === 1 ? "1 Vertrag" : `${count} Verträge`;
}

/** "1 Rechnung" / "12 Rechnungen". */
export function invoiceCountLabel(count: number): string {
  return count === 1 ? "1 Rechnung" : `${count} Rechnungen`;
}
