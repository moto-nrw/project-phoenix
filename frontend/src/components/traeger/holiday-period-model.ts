/**
 * The editable draft of a holiday period (#4659) and the checks the form runs
 * before it sends. Field names match the backend's `errors[].field`, so a
 * refusal from the server lands at the same control as a local check.
 */

import { isValidISODate, parseISODate } from "~/lib/date-helpers";
import type {
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
}

export interface WeekDraft {
  readonly key: string;
  readonly id?: string;
  readonly startsOn: string;
  readonly endsOn: string;
  readonly hostSchoolId: string;
  readonly timeModels: readonly TimeModelDraft[];
}

export interface HolidayPeriodDraft {
  readonly name: string;
  readonly registrationOpensOn: string;
  readonly registrationClosesOn: string;
  readonly binding: boolean;
  readonly notes: string;
  readonly weeks: readonly WeekDraft[];
}

const MAX_WEEK_DAYS = 7;
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
  };
}

export function emptyWeek(): WeekDraft {
  return {
    key: nextKey("week"),
    startsOn: "",
    endsOn: "",
    hostSchoolId: "",
    timeModels: [emptyTimeModel()],
  };
}

export function emptyHolidayPeriod(): HolidayPeriodDraft {
  return {
    name: "",
    registrationOpensOn: "",
    registrationClosesOn: "",
    binding: true,
    notes: "",
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
  };
}

export function draftFromPeriod(period: HolidayPeriod): HolidayPeriodDraft {
  return {
    name: period.name,
    registrationOpensOn: period.registrationOpensOn,
    registrationClosesOn: period.registrationClosesOn,
    binding: period.binding,
    notes: period.notes,
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
    weeks: draft.weeks.map((week) => ({
      id: week.id,
      startsOn: week.startsOn,
      endsOn: week.endsOn,
      hostSchoolId: week.hostSchoolId,
      timeModels: week.timeModels.map((model) => ({
        id: model.id,
        name: model.name.trim(),
        startsAt: model.startsAt,
        endsAt: model.endsAt,
        priceCents: euroInputToCents(model.price) ?? 0,
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
      errors[`${field}.starts_at`] = "Bitte im Format 08:00 eingeben.";
    }
    if (!endsValid) {
      errors[`${field}.ends_at`] = "Bitte im Format 13:00 eingeben.";
    } else if (startsValid && model.endsAt <= model.startsAt) {
      errors[`${field}.ends_at`] = "Das Ende liegt nicht nach dem Beginn.";
    }
    const cents = euroInputToCents(model.price);
    if (cents === null || cents > MAX_PRICE_CENTS) {
      errors[`${field}.price_cents`] =
        "Bitte einen Preis in Euro eingeben, zum Beispiel 45,00.";
    }
  });
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
        weekDatesValid(week) &&
        weekDatesValid(other) &&
        week.startsOn <= other.endsOn &&
        other.startsOn <= week.endsOn,
    );
    if (overlap >= 0 && !errors[`${prefix}.starts_on`]) {
      errors[`${prefix}.starts_on`] =
        `Diese Woche überschneidet sich mit Woche ${overlap + 1}.`;
    }
    checkTimeModels(week, prefix, errors);
  });
  return errors;
}

/** Publishing needs at least one week, each with a time model. */
export function holidayPeriodComplete(period: HolidayPeriod): boolean {
  return (
    period.weeks.length > 0 &&
    period.weeks.every((week) => week.timeModels.length > 0)
  );
}
