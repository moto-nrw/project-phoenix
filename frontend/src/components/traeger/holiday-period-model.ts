/**
 * The editable draft of a holiday period (#4659) and the checks the form runs
 * before it sends. Field names match the backend's `errors[].field`, so a
 * refusal from the server lands at the same control as a local check.
 */

import { isValidISODate, parseISODate } from "~/lib/date-helpers";
import type {
  HolidayAllocationMode,
  HolidayBookingUnit,
  HolidayPeriod,
  HolidayPeriodInput,
  HolidayTimeModel,
  HolidayWeek,
} from "~/lib/traeger-api";

export interface TimeModelDraft {
  /** Stable React key; never sent. */
  readonly key: string;
  readonly id?: string;
  readonly name: string;
  readonly startsAt: string;
  readonly endsAt: string;
  /** Price as typed, in euro: "45", "45,5" or "45,50". */
  readonly price: string;
  /** No longer bookable; stays in the week for the days booked on it. */
  readonly archived: boolean;
  /** Bookings name it: it cannot be removed, only archived (#5994). */
  readonly protected: boolean;
}

export interface WeekDraft {
  readonly key: string;
  readonly id?: string;
  readonly startsOn: string;
  readonly endsOn: string;
  readonly hostSchoolId: string;
  readonly timeModels: readonly TimeModelDraft[];
  /** Whole week or single days (#4701); the price is per week or per day. */
  readonly bookingUnit?: HolidayBookingUnit;
  /** The week offers lunch (#5631). */
  readonly lunchOffered?: boolean;
  /** Price of one lunch as typed, in euro; kept while the offer is off. */
  readonly lunchPrice?: string;
  /** A booking has lunch: the offer stays, only its price changes. */
  readonly lunchBooked?: boolean;
}

export interface HolidayPeriodDraft {
  readonly name: string;
  readonly registrationOpensOn: string;
  readonly registrationClosesOn: string;
  readonly binding: boolean;
  readonly notes: string;
  /** The Träger's text in the acceptance mail to the parents (#6672). */
  readonly acceptanceMailText: string;
  readonly holidayWishRule: boolean;
  /** Care days per child as typed; empty is no bound (#4701). */
  readonly maxDaysPerChild: string;
  /** In the order of arrival or by the office after criteria (#4700). */
  readonly allocationMode: HolidayAllocationMode;
  readonly weeks: readonly WeekDraft[];
}

const MAX_WEEK_DAYS = 7;
/** The backend's bound of care days per child (holiday_days.go). */
const MAX_DAYS_PER_CHILD = 366;
/** The backend's bound of the acceptance mail text (holiday_acceptance.go). */
export const MAX_ACCEPTANCE_MAIL_TEXT = 2000;
/** The backend's limits of one period (holiday_periods.go). */
export const MAX_HOLIDAY_WEEKS = 26;
export const MAX_TIME_MODELS_PER_WEEK = 10;
const MAX_PRICE_CENTS = 1_000_000;

let keySequence = 0;
function nextKey(prefix: string): string {
  keySequence += 1;
  return `${prefix}-${keySequence}`;
}

export function emptyTimeModel(): TimeModelDraft {
  return {
    key: nextKey("time-model"),
    name: "",
    startsAt: "",
    endsAt: "",
    price: "",
    archived: false,
    protected: false,
  };
}

export function emptyWeek(): WeekDraft {
  return {
    key: nextKey("week"),
    startsOn: "",
    endsOn: "",
    hostSchoolId: "",
    timeModels: [emptyTimeModel()],
    bookingUnit: "week",
  };
}

/** A week is booked once a booking or a rebooking names a time model. */
export function weekBooked(week: WeekDraft): boolean {
  return week.timeModels.some((model) => model.protected);
}

export function emptyHolidayPeriod(): HolidayPeriodDraft {
  return {
    name: "",
    registrationOpensOn: "",
    registrationClosesOn: "",
    binding: true,
    notes: "",
    acceptanceMailText: "",
    holidayWishRule: false,
    maxDaysPerChild: "",
    allocationMode: "arrival",
    weeks: [emptyWeek()],
  };
}

/** Cents → "45,00", the value the price field shows. */
export function centsToEuroInput(cents: number): string {
  return (cents / 100).toFixed(2).replace(".", ",");
}

/** "45", "45,5", "45,50" or "45.50" → 4550 cent; anything else is null. */
export function euroInputToCents(value: string): number | null {
  const match = /^(\d{1,7})(?:[.,](\d{1,2}))?$/.exec(value.trim());
  if (!match) return null;
  const euros = Number(match[1]);
  const cents = Number((match[2] ?? "").padEnd(2, "0"));
  return euros * 100 + cents;
}

function timeModelDraft(model: HolidayTimeModel): TimeModelDraft {
  return {
    key: nextKey("time-model"),
    id: model.id,
    name: model.name,
    startsAt: model.startsAt,
    endsAt: model.endsAt,
    price: centsToEuroInput(model.priceCents),
    archived: model.archived ?? false,
    protected: model.protected ?? false,
  };
}

function weekDraft(week: HolidayWeek): WeekDraft {
  return {
    key: nextKey("week"),
    id: week.id,
    startsOn: week.startsOn,
    endsOn: week.endsOn,
    hostSchoolId: week.hostSchoolId,
    timeModels: week.timeModels.map(timeModelDraft),
    bookingUnit: week.bookingUnit ?? "week",
    lunchOffered: week.lunchPriceCents != null,
    lunchPrice:
      week.lunchPriceCents == null
        ? ""
        : centsToEuroInput(week.lunchPriceCents),
    lunchBooked: week.lunchBooked ?? false,
  };
}

export function draftFromPeriod(period: HolidayPeriod): HolidayPeriodDraft {
  return {
    name: period.name,
    registrationOpensOn: period.registrationOpensOn,
    registrationClosesOn: period.registrationClosesOn,
    binding: period.binding,
    notes: period.notes,
    acceptanceMailText: period.acceptanceMailText ?? "",
    holidayWishRule: period.holidayWishRule,
    maxDaysPerChild:
      period.maxDaysPerChild == null ? "" : String(period.maxDaysPerChild),
    allocationMode: period.allocationMode ?? "arrival",
    weeks: period.weeks.map(weekDraft),
  };
}

/** The request body; call only after `validateHolidayPeriod` found nothing. */
export function inputFromDraft(draft: HolidayPeriodDraft): HolidayPeriodInput {
  return {
    name: draft.name.trim(),
    registrationOpensOn: draft.registrationOpensOn,
    registrationClosesOn: draft.registrationClosesOn,
    binding: draft.binding,
    notes: draft.notes.trim(),
    acceptanceMailText: draft.acceptanceMailText.trim(),
    holidayWishRule: draft.holidayWishRule,
    maxDaysPerChild: draft.maxDaysPerChild.trim()
      ? Number(draft.maxDaysPerChild.trim())
      : null,
    allocationMode: draft.allocationMode,
    weeks: draft.weeks.map((week) => ({
      id: week.id,
      startsOn: week.startsOn,
      endsOn: week.endsOn,
      hostSchoolId: week.hostSchoolId,
      bookingUnit: week.bookingUnit,
      lunchPriceCents: week.lunchOffered
        ? (euroInputToCents(week.lunchPrice ?? "") ?? 0)
        : null,
      timeModels: week.timeModels.map((model) => ({
        id: model.id,
        name: model.name.trim(),
        startsAt: model.startsAt,
        endsAt: model.endsAt,
        priceCents: euroInputToCents(model.price) ?? 0,
        archived: model.archived,
      })),
    })),
  };
}

const WALL_CLOCK = /^([01]\d|2[0-3]):[0-5]\d$/;

function daysBetween(from: string, to: string): number {
  const ms = parseISODate(to).getTime() - parseISODate(from).getTime();
  return Math.round(ms / 86_400_000);
}

function checkRegistration(
  draft: HolidayPeriodDraft,
  errors: Record<string, string>,
) {
  if (!isValidISODate(draft.registrationOpensOn)) {
    errors.registration_opens_on = "Bitte wählen Sie den Anmeldebeginn.";
  }
  if (!isValidISODate(draft.registrationClosesOn)) {
    errors.registration_closes_on = "Bitte wählen Sie den Anmeldeschluss.";
  } else if (
    isValidISODate(draft.registrationOpensOn) &&
    draft.registrationClosesOn < draft.registrationOpensOn
  ) {
    errors.registration_closes_on =
      "Der Anmeldeschluss liegt vor dem Anmeldebeginn.";
  }
}

function weekDatesValid(week: WeekDraft): boolean {
  return (
    isValidISODate(week.startsOn) &&
    isValidISODate(week.endsOn) &&
    week.startsOn <= week.endsOn
  );
}

function checkWeekDates(
  week: WeekDraft,
  prefix: string,
  errors: Record<string, string>,
) {
  if (!isValidISODate(week.startsOn)) {
    errors[`${prefix}.starts_on`] = "Bitte wählen Sie den ersten Tag.";
  }
  if (!isValidISODate(week.endsOn)) {
    errors[`${prefix}.ends_on`] = "Bitte wählen Sie den letzten Tag.";
    return;
  }
  if (!isValidISODate(week.startsOn)) return;
  if (week.endsOn < week.startsOn) {
    errors[`${prefix}.ends_on`] = "Der letzte Tag liegt vor dem ersten Tag.";
  } else if (daysBetween(week.startsOn, week.endsOn) >= MAX_WEEK_DAYS) {
    errors[`${prefix}.ends_on`] = "Eine Woche hat höchstens 7 Tage.";
  }
}

function checkTimeModels(
  week: WeekDraft,
  prefix: string,
  errors: Record<string, string>,
) {
  const names = new Set<string>();
  week.timeModels.forEach((model, index) => {
    const field = `${prefix}.time_models.${index}`;
    const name = model.name.trim().toLowerCase();
    if (!name) {
      errors[`${field}.name`] = "Bitte geben Sie einen Namen ein.";
    } else if (names.has(name)) {
      errors[`${field}.name`] = "Diesen Namen gibt es in der Woche schon.";
    }
    names.add(name);
    const startsValid = WALL_CLOCK.test(model.startsAt);
    const endsValid = WALL_CLOCK.test(model.endsAt);
    if (!startsValid) {
      errors[`${field}.starts_at`] = model.startsAt
        ? "Bitte geben Sie die Zeit im Format 08:00 ein."
        : "Bitte tragen Sie den Beginn ein.";
    }
    if (!endsValid) {
      errors[`${field}.ends_at`] = model.endsAt
        ? "Bitte geben Sie die Zeit im Format 13:00 ein."
        : "Bitte tragen Sie das Ende ein.";
    } else if (startsValid && model.endsAt <= model.startsAt) {
      errors[`${field}.ends_at`] = "Das Ende liegt nicht nach dem Beginn.";
    }
    const cents = euroInputToCents(model.price);
    if (cents === null || cents > MAX_PRICE_CENTS) {
      errors[`${field}.price_cents`] =
        "Bitte geben Sie einen Preis in Euro ein, zum Beispiel 45,00.";
    }
  });
}

/**
 * The lunches of a booking of the whole week and what they cost together,
 * or null without a valid price or without care days (#5631).
 */
export function weekLunchTotal(
  week: WeekDraft,
  careDays: number,
): { readonly lunches: number; readonly cents: number } | null {
  const cents = euroInputToCents(week.lunchPrice ?? "");
  if (!week.lunchOffered || cents === null || careDays <= 0) return null;
  return { lunches: careDays, cents: cents * careDays };
}

/**
 * The checks of the form, keyed by field name. Empty means the draft can be
 * sent. A draft may have weeks without time models; publishing needs them.
 */
export function validateHolidayPeriod(
  draft: HolidayPeriodDraft,
): Record<string, string> {
  const errors: Record<string, string> = {};
  if (!draft.name.trim()) {
    errors.name = "Bitte geben Sie einen Namen ein.";
  }
  checkRegistration(draft, errors);
  const limit = draft.maxDaysPerChild.trim();
  if (
    limit &&
    (!/^\d{1,3}$/.test(limit) ||
      Number(limit) < 1 ||
      Number(limit) > MAX_DAYS_PER_CHILD)
  ) {
    errors.max_days_per_child = `Bitte eine Zahl von 1 bis ${MAX_DAYS_PER_CHILD} eingeben oder das Feld leer lassen.`;
  }
  // Characters as the backend counts them, not UTF-16 units.
  if ([...draft.acceptanceMailText.trim()].length > MAX_ACCEPTANCE_MAIL_TEXT) {
    errors.acceptance_mail_text = `Der Text darf höchstens ${MAX_ACCEPTANCE_MAIL_TEXT} Zeichen lang sein.`;
  }
  draft.weeks.forEach((week, index) => {
    const prefix = `weeks.${index}`;
    checkWeekDates(week, prefix, errors);
    if (!week.hostSchoolId) {
      errors[`${prefix}.host_school_id`] =
        "Bitte wählen Sie die ausrichtende OGS.";
    }
    const overlap = draft.weeks.findIndex(
      (other, otherIndex) =>
        otherIndex < index &&
        week.hostSchoolId !== "" &&
        week.hostSchoolId === other.hostSchoolId &&
        weekDatesValid(week) &&
        weekDatesValid(other) &&
        week.startsOn <= other.endsOn &&
        other.startsOn <= week.endsOn,
    );
    if (overlap >= 0 && !errors[`${prefix}.starts_on`]) {
      errors[`${prefix}.starts_on`] =
        `Diese Woche überschneidet sich mit Woche ${overlap + 1} an derselben Schule.`;
    }
    checkTimeModels(week, prefix, errors);
    if (week.lunchOffered) {
      const cents = euroInputToCents(week.lunchPrice ?? "");
      if (cents === null || cents > MAX_PRICE_CENTS) {
        errors[`${prefix}.lunch_price_cents`] =
          "Bitte geben Sie den Preis für ein Essen in Euro ein, zum Beispiel 4,30.";
      }
    }
  });
  return errors;
}

/**
 * Publishing needs at least one week, each with a time model that is not
 * archived.
 */
export function holidayPeriodComplete(period: HolidayPeriod): boolean {
  return (
    period.weeks.length > 0 &&
    period.weeks.every((week) =>
      week.timeModels.some((model) => !model.archived),
    )
  );
}
