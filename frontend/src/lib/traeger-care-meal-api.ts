/**
 * Client of the Mensa of the monthly run (#4686): how the Träger bills the
 * Mensa of each school (flat per child and month or per meal) and the Mensa
 * parts of a run (positions, months without a release, children not
 * billed). Money stays whole cent; backend int64 IDs leave as strings;
 * calendar dates stay "YYYY-MM-DD".
 */

import { traegerRequest } from "./traeger-api";

export type CareMealMode = "flat" | "per_meal";

export interface CareMealTariff {
  readonly id: string;
  readonly schoolId: string;
  readonly validFrom: string;
  /** The day before the next price of the school; null while none follows. */
  readonly validUntil: string | null;
  readonly mode: CareMealMode;
  readonly amountCents: number;
}

export interface CareMealTariffs {
  readonly schools: readonly { readonly id: string; readonly name: string }[];
  readonly tariffs: readonly CareMealTariff[];
  /** Per school the last Mensa month an issued invoice bills. */
  readonly billedUntil: Readonly<Record<string, string>>;
}

/** One Mensa month of a child on an invoice. */
export interface CareMealPosition {
  readonly mealMonth: string;
  readonly mode: CareMealMode;
  readonly quantity: number;
  readonly unitCents: number;
  readonly amountCents: number;
}

/** An ended month a school has not released yet. */
export interface CareMealGap {
  readonly schoolId: string;
  readonly schoolName: string;
  readonly month: string;
}

/** A released Mensa month of a child the run does not bill. */
export interface CareMealUnbilled {
  readonly schoolId: string;
  readonly schoolName: string;
  readonly studentId: string;
  readonly childFirstName: string;
  readonly childLastName: string;
  readonly month: string;
  readonly mealDays: number;
  readonly reason: "no_contract" | "contract_billed";
}

export interface BackendMealPosition {
  meal_month: string;
  mode: CareMealMode;
  quantity: number;
  unit_cents: number;
  amount_cents: number;
}

export interface BackendMealGap {
  school_id: number;
  school_name: string;
  month: string;
}

export interface BackendMealUnbilled {
  school_id: number;
  school_name: string;
  student_id: number;
  child_first_name: string;
  child_last_name: string;
  month: string;
  meal_days: number;
  reason: CareMealUnbilled["reason"];
}

interface BackendMealTariffs {
  schools: { id: number; name: string }[] | null;
  tariffs:
    | {
        id: number;
        school_id: number;
        valid_from: string;
        valid_until: string | null;
        mode: CareMealMode;
        amount_cents: number;
      }[]
    | null;
  billed_until: Record<string, string> | null;
}

const MEAL_TARIFFS_PATH = "/api/traeger/care-meal-tariffs";

function id(value: number): string {
  return value ? String(value) : "";
}

export function mealPositionsFromBackend(
  positions: readonly BackendMealPosition[] | null | undefined,
): CareMealPosition[] {
  return (positions ?? []).map((position) => ({
    mealMonth: position.meal_month,
    mode: position.mode,
    quantity: position.quantity,
    unitCents: position.unit_cents,
    amountCents: position.amount_cents,
  }));
}

export function mealGapsFromBackend(
  gaps: readonly BackendMealGap[] | null | undefined,
): CareMealGap[] {
  return (gaps ?? []).map((gap) => ({
    schoolId: id(gap.school_id),
    schoolName: gap.school_name,
    month: gap.month,
  }));
}

export function mealUnbilledFromBackend(
  entries: readonly BackendMealUnbilled[] | null | undefined,
): CareMealUnbilled[] {
  return (entries ?? []).map((entry) => ({
    schoolId: id(entry.school_id),
    schoolName: entry.school_name,
    studentId: id(entry.student_id),
    childFirstName: entry.child_first_name,
    childLastName: entry.child_last_name,
    month: entry.month,
    mealDays: entry.meal_days,
    reason: entry.reason,
  }));
}

function mealTariffsFromBackend(body: BackendMealTariffs): CareMealTariffs {
  return {
    schools: (body.schools ?? []).map((school) => ({
      id: id(school.id),
      name: school.name,
    })),
    tariffs: (body.tariffs ?? []).map((tariff) => ({
      id: id(tariff.id),
      schoolId: id(tariff.school_id),
      validFrom: tariff.valid_from,
      validUntil: tariff.valid_until,
      mode: tariff.mode,
      amountCents: tariff.amount_cents,
    })),
    billedUntil: body.billed_until ?? {},
  };
}

export async function fetchCareMealTariffs(): Promise<CareMealTariffs> {
  return mealTariffsFromBackend(
    await traegerRequest<BackendMealTariffs>(MEAL_TARIFFS_PATH),
  );
}

/** Sets how the Mensa of a school is billed from `validFrom` on. */
export async function saveCareMealTariff(input: {
  readonly schoolId: string;
  readonly validFrom: string;
  readonly mode: CareMealMode;
  readonly amountCents: number;
}): Promise<CareMealTariffs> {
  return mealTariffsFromBackend(
    await traegerRequest<BackendMealTariffs>(MEAL_TARIFFS_PATH, {
      method: "PUT",
      body: {
        school_id: Number(input.schoolId),
        valid_from: input.validFrom,
        mode: input.mode,
        amount_cents: input.amountCents,
      },
    }),
  );
}

/** Removes the Mensa price of a school that starts on `validFrom`. */
export async function deleteCareMealTariff(
  schoolId: string,
  validFrom: string,
): Promise<CareMealTariffs> {
  return mealTariffsFromBackend(
    await traegerRequest<BackendMealTariffs>(
      `${MEAL_TARIFFS_PATH}/${encodeURIComponent(schoolId)}/${encodeURIComponent(validFrom)}`,
      { method: "DELETE" },
    ),
  );
}

/** The Mensa price of a school in a month, if any. */
export function mealTariffOn(
  tariffs: readonly CareMealTariff[],
  schoolId: string,
  month: string,
): CareMealTariff | null {
  let found: CareMealTariff | null = null;
  for (const tariff of tariffs) {
    if (
      tariff.schoolId === schoolId &&
      tariff.validFrom <= month &&
      (!found || tariff.validFrom > found.validFrom)
    ) {
      found = tariff;
    }
  }
  return found;
}
