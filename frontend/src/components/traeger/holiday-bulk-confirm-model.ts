// Mehrere Ferienanmeldungen auf einmal bestätigen (#6674). Bestätigt werden
// nur Anmeldungen, die ohne weitere Angabe bestätigt werden können: Die
// Familie bezahlt, und die Debitorennummer ist da oder keine Pflicht. Der
// Rest bleibt mit seinem Grund stehen und wird einzeln bestätigt. Jede
// Anmeldung läuft durch dieselbe Bestätigung wie auf ihrer Seite, mit eigener
// Prüfung, eigenem Protokoll und eigener Annahme-Mail.

import { ApiError } from "~/lib/api-error";
import type { ErrorCode } from "~/lib/error-codes.generated";
import type { HolidayRegistrationSummary } from "~/lib/traeger-holiday-registrations-api";
import type { HolidaySiblingGroup } from "~/lib/traeger-sibling-groups-api";

/** Why a registration stays open. */
export type HolidayBulkSkipReason =
  // Known before the run, from the list.
  | "not_received"
  | "place_open"
  | "payer_open"
  | "duplicate"
  | "debtor_number_missing"
  // The backend refused this one registration.
  | "changed"
  | "failed"
  // The run ended before this registration: the office stopped it, or no
  // further confirmation could succeed.
  | "stopped"
  | "interrupted"
  // The request went out but no answer came: it may be confirmed.
  | "unclear";

export interface HolidayBulkSkip {
  readonly registration: HolidayRegistrationSummary;
  readonly reason: HolidayBulkSkipReason;
  /** The catalog text of a refusal no reason names (`failed`). */
  readonly note?: string;
}

/** "1 Anmeldung", "3 Anmeldungen". */
export function registrationCount(count: number): string {
  return count === 1 ? "1 Anmeldung" : `${count} Anmeldungen`;
}

/** A child of the registration is maybe registered twice in the period. */
export function hasPossibleDuplicate(
  registration: HolidayRegistrationSummary,
): boolean {
  return registration.children.some(
    (child) => child.duplicateRegistrationIds.length > 0,
  );
}

export interface HolidayBulkPlan {
  readonly confirmable: readonly HolidayRegistrationSummary[];
  readonly skipped: readonly HolidayBulkSkip[];
}

interface PlanContext {
  /** The Träger asks for the debtor number at the confirmation (#5621). */
  readonly debtorNumberRequired: boolean;
  readonly groups: readonly HolidaySiblingGroup[];
  /** Every registration the list shows, for the number of a group. */
  readonly shown: readonly HolidayRegistrationSummary[];
}

interface PlanLookup {
  readonly debtorNumberRequired: boolean;
  readonly groups: readonly HolidaySiblingGroup[];
  readonly shownById: ReadonlyMap<string, HolidayRegistrationSummary>;
}

/**
 * Whether a registration in a sibling group can take its group's debtor
 * number (#5628). The backend takes it the same way; a group the list shows
 * only in part is tried, and the backend refuses it without a number.
 */
function groupNumberPossible(
  registration: HolidayRegistrationSummary,
  lookup: PlanLookup,
): boolean {
  const group = lookup.groups.find((entry) =>
    entry.registrationIds.includes(registration.id),
  );
  if (!group) return false;
  return group.registrationIds.some((id) => {
    if (id === registration.id) return false;
    const other = lookup.shownById.get(id);
    return !other || other.debtorNumber.trim() !== "";
  });
}

function skipReason(
  registration: HolidayRegistrationSummary,
  lookup: PlanLookup,
): HolidayBulkSkipReason | null {
  if (registration.status !== "received") return "not_received";
  if (
    registration.children.some((child) =>
      child.bookings.some((booking) => booking.placement === "pending"),
    )
  ) {
    return "place_open";
  }
  // The parents state benefits: the office checks the proof and chooses
  // who pays. Confirming with the family as payer would bill them unseen.
  if (registration.benefitsDeclared) return "payer_open";
  // Two registrations of one child would both be billed and both mailed.
  if (hasPossibleDuplicate(registration)) return "duplicate";
  if (
    lookup.debtorNumberRequired &&
    registration.debtorNumber.trim() === "" &&
    !groupNumberPossible(registration, lookup)
  ) {
    return "debtor_number_missing";
  }
  return null;
}

/** Splits the selection into what is confirmed and what stays open. */
export function planHolidayBulkConfirmation(
  registrations: readonly HolidayRegistrationSummary[],
  context: PlanContext,
): HolidayBulkPlan {
  const lookup: PlanLookup = {
    debtorNumberRequired: context.debtorNumberRequired,
    groups: context.groups,
    shownById: new Map(context.shown.map((entry) => [entry.id, entry])),
  };
  const confirmable: HolidayRegistrationSummary[] = [];
  const skipped: HolidayBulkSkip[] = [];
  for (const registration of registrations) {
    const reason = skipReason(registration, lookup);
    if (reason) skipped.push({ registration, reason });
    else confirmable.push(registration);
  }
  return { confirmable, skipped };
}

const REFUSALS: Partial<Record<ErrorCode, HolidayBulkSkipReason>> = {
  "enrollment.holiday_allocation_pending": "place_open",
  "parent_billing.debtor_number_invalid": "debtor_number_missing",
  "enrollment.holiday_registration_state_conflict": "changed",
  "enrollment.holiday_registration_entry_not_found": "changed",
};

/**
 * What a failed confirmation means for the run. A refusal keeps only its
 * registration open (`stop: false`). Without an answer, after the end of the
 * session or without the right, no further confirmation can succeed: the run
 * stops, and without an answer the registration may be confirmed after all.
 */
export function bulkConfirmOutcome(error: unknown): {
  readonly reason: HolidayBulkSkipReason;
  readonly stop: boolean;
} {
  if (!(error instanceof ApiError)) {
    return { reason: "interrupted", stop: true };
  }
  // A proxy timeout can arrive after the backend committed the confirmation.
  if (error.noResponse || error.code === "general.unavailable") {
    return { reason: "unclear", stop: true };
  }
  if (error.status === 401 || error.status === 403) {
    return { reason: "interrupted", stop: true };
  }
  return {
    reason: (error.code && REFUSALS[error.code]) ?? "failed",
    stop: false,
  };
}
