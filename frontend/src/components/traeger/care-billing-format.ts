/**
 * Display helpers of the monthly contribution (#4684): month names, weekday
 * labels and the price matrix of one school at a month. Calendar months stay
 * "YYYY-MM-01" strings; no Date crosses a time zone here.
 */

import type {
  CareModule,
  CarePriceForm,
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
  // „am 1.“ stays together on one line.
  version_on_first: "Voller Monat nach dem Vertrag am\u00a01.",
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
    ...new Set([
      ...data.tariffs
        .filter((tariff) => tariff.schoolId === schoolId)
        .map((tariff) => tariff.validFrom),
      ...(data.lists ?? [])
        .filter((list) => list.schoolId === schoolId)
        .map((list) => list.validFrom),
    ]),
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

/** Every supported scale step: 1 to 7 days per week. */
export const SCALE_STEPS = [1, 2, 3, 4, 5, 6, 7] as const;

/** "1 Tag" / "3 Tage". */
export function daysLabel(days: number): string {
  return days === 1 ? "1 Tag" : `${days} Tage`;
}

/** The two price forms as the price list names them (#6196). */
export const PRICE_FORM_LABELS: Record<CarePriceForm, string> = {
  weekday: "Preis je Wochentag",
  days_per_week: "Staffel nach Tagen je Woche",
};

const MONTH_SHORT = [
  "Jan",
  "Feb",
  "Mär",
  "Apr",
  "Mai",
  "Jun",
  "Jul",
  "Aug",
  "Sep",
  "Okt",
  "Nov",
  "Dez",
] as const;

/** "August" for 8. */
export function monthName(month: number): string {
  return MONTH_NAMES[month - 1] ?? "";
}

/** "Jan" for 1. */
export function monthShort(month: number): string {
  return MONTH_SHORT[month - 1] ?? "";
}

/** "August" / "Juli und August" / "Juli, August und Dezember". */
export function freeMonthsLabel(months: readonly number[]): string {
  const names = [...months].sort((a, b) => a - b).map(monthName);
  if (names.length <= 1) return names.join("");
  return `${names.slice(0, -1).join(", ")} und ${names.at(-1)}`;
}

/** The months the price list of a school at `month` bills nothing for. */
export function freeMonthsAt(
  data: CareTariffs,
  schoolId: string,
  month: string,
): readonly number[] {
  const lists = (data.lists ?? [])
    .filter((list) => list.schoolId === schoolId && list.validFrom <= month)
    .sort((a, b) => a.validFrom.localeCompare(b.validFrom));
  return lists.at(-1)?.freeMonths ?? [];
}

/** One module of a school with its prices at a month. */
export interface PriceRow {
  readonly careOfferingId: string;
  readonly moduleName: string;
  readonly endsAt: string;
  readonly contractCount: number;
  readonly bookedWeekdays: readonly CareWeekday[];
  readonly bookedDaysPerWeek: readonly number[];
  /** How the module is priced at the month. */
  readonly form: CarePriceForm;
  /** The price excludes the meal (the Mensa bills it). */
  readonly mealExtra: boolean;
  /** Monthly price in cent per weekday; missing when none holds. */
  readonly prices: Partial<Record<CareWeekday, number>>;
  /** Price for every further child per weekday; missing: as the first. */
  readonly siblingPrices: Partial<Record<CareWeekday, number>>;
  /** Monthly price in cent per number of days per week. */
  readonly steps: Partial<Record<number, number>>;
  /** Price for every further child per number of days. */
  readonly siblingSteps: Partial<Record<number, number>>;
}

type Tariff = CareTariffs["tariffs"][number];

function formOf(tariff: Tariff): CarePriceForm {
  return (tariff.daysPerWeek ?? null) === null ? "weekday" : "days_per_week";
}

/**
 * The modules of a school with their prices at `month`. A module has the
 * form of its latest price that started by then; prices of the other form
 * do not hold (as the monthly run reads them).
 */
export function priceRowsAt(
  data: CareTariffs,
  schoolId: string,
  month: string,
): PriceRow[] {
  const modules = data.modules.filter((module) => module.schoolId === schoolId);
  return modules.map((module: CareModule) => {
    const started = data.tariffs
      .filter(
        (tariff) =>
          tariff.schoolId === schoolId &&
          tariff.careOfferingId === module.careOfferingId &&
          tariff.validFrom <= month &&
          (tariff.validUntil === null || tariff.validUntil >= month),
      )
      .sort((a, b) => a.validFrom.localeCompare(b.validFrom));
    const latest = started.at(-1);
    const form: CarePriceForm = latest ? formOf(latest) : "weekday";
    const prices: Partial<Record<CareWeekday, number>> = {};
    const siblingPrices: Partial<Record<CareWeekday, number>> = {};
    const steps: Partial<Record<number, number>> = {};
    const siblingSteps: Partial<Record<number, number>> = {};
    for (const tariff of started) {
      if (formOf(tariff) !== form) continue;
      const days = tariff.daysPerWeek ?? null;
      const sibling = tariff.siblingCents ?? null;
      if (days !== null) {
        steps[days] = tariff.monthlyCents;
        if (sibling !== null) siblingSteps[days] = sibling;
      } else if (tariff.weekday !== "") {
        prices[tariff.weekday] = tariff.monthlyCents;
        if (sibling !== null) siblingPrices[tariff.weekday] = sibling;
      }
    }
    return {
      careOfferingId: module.careOfferingId,
      moduleName: module.moduleName,
      endsAt: module.endsAt,
      contractCount: module.contractCount,
      bookedWeekdays: module.bookedWeekdays,
      bookedDaysPerWeek: module.bookedDaysPerWeek,
      form,
      mealExtra: latest?.mealExtra ?? false,
      prices,
      siblingPrices,
      steps,
      siblingSteps,
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

/**
 * What a row books without a price at the month: the weekdays ("sat"), or
 * for a scale the actual booked sizes ("3 Tage").
 */
export function missingPrices(row: PriceRow): string[] {
  if (row.form === "days_per_week") {
    return row.bookedDaysPerWeek
      .filter((days) => row.steps[days] === undefined)
      .map(daysLabel);
  }
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

/**
 * Short facts joined by „ · “ that wrap only between facts, after the dot:
 * a date, a count or an amount never breaks inside, and no line starts with
 * „·“. For short facts only; a name may need to wrap.
 */
export function factLine(
  facts: readonly (string | null | undefined | false)[],
): string {
  return facts
    .filter((fact): fact is string => Boolean(fact))
    .map((fact) => fact.replaceAll(" ", "\u00A0"))
    .join("\u00A0· ");
}
