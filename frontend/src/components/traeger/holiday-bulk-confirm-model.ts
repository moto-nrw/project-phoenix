// Mehrere Ferienanmeldungen auf einmal bestätigen (#6674). Bestätigt werden
// nur Anmeldungen, die ohne weitere Angabe bestätigt werden können: Die
// Familie bezahlt, und die Debitorennummer ist da oder keine Pflicht. Der
// Rest bleibt mit seinem Grund stehen und wird einzeln bestätigt. Jede
// Anmeldung läuft durch dieselbe Bestätigung wie auf ihrer Seite, mit eigener
// Prüfung, eigenem Protokoll und eigener Annahme-Mail.

import { ApiError } from "~/lib/api-error";
import type { HolidayRegistrationSummary } from "~/lib/traeger-holiday-registrations-api";
import type { HolidaySiblingGroup } from "~/lib/traeger-sibling-groups-api";

/**
 * Why a registration stays open. In the order the office works through
 * them: a decided place before the payer, the payer before the number.
 */
export type HolidayBulkSkipReason =
  | "not_received"
  | "place_open"
  | "payer_open"
  | "duplicate"
  | "debtor_number_missing"
  | "changed"
  | "failed"
  // The run stopped before this registration: the office stopped it, or no
  // further confirmation could succeed.
  | "stopped";

export interface HolidayBulkSkip {
  readonly registration: HolidayRegistrationSummary;
  readonly reason: HolidayBulkSkipReason;
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
  readonly known: readonly HolidayRegistrationSummary[];
}

/**
 * Whether a registration in a sibling group can take its group's debtor
 * number (#5628). The backend takes it the same way; a group the list shows
 * only in part is tried, and the backend refuses it without a number.
 */
function groupNumberPossible(
  registration: HolidayRegistrationSummary,
  context: PlanContext,
): boolean {
  const group = context.groups.find((entry) =>
    entry.registrationIds.includes(registration.id),
  );
  if (!group) return false;
  const others = group.registrationIds.filter((id) => id !== registration.id);
  const shown = new Map(context.known.map((entry) => [entry.id, entry]));
  return others.some((id) => {
    const other = shown.get(id);
    return !other || other.debtorNumber.trim() !== "";
  });
}

function skipReason(
  registration: HolidayRegistrationSummary,
  context: PlanContext,
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
  if (
    registration.children.some(
      (child) => child.duplicateRegistrationIds.length > 0,
    )
  ) {
    return "duplicate";
  }
  if (
    context.debtorNumberRequired &&
    registration.debtorNumber.trim() === "" &&
    !groupNumberPossible(registration, context)
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
  const confirmable: HolidayRegistrationSummary[] = [];
  const skipped: HolidayBulkSkip[] = [];
  for (const registration of registrations) {
    const reason = skipReason(registration, context);
    if (reason) skipped.push({ registration, reason });
    else confirmable.push(registration);
  }
  return { confirmable, skipped };
}

const REFUSALS: Partial<Record<string, HolidayBulkSkipReason>> = {
  "enrollment.holiday_allocation_pending": "place_open",
  "parent_billing.debtor_number_invalid": "debtor_number_missing",
  "enrollment.holiday_registration_state_conflict": "changed",
  "enrollment.holiday_registration_entry_not_found": "changed",
};

/**
 * What a failed confirmation means for the run: the registration stays open
 * with its reason, or the run stops because no further one can succeed (no
 * answer, the session ended, the right is gone).
 */
export function bulkConfirmOutcome(
  error: unknown,
): { readonly reason: HolidayBulkSkipReason } | "stop" {
  if (!(error instanceof ApiError) || error.noResponse) return "stop";
  if (error.status === 401 || error.status === 403) return "stop";
  return { reason: (error.code && REFUSALS[error.code]) ?? "failed" };
}
