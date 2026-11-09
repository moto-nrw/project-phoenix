import type { DecisionStatus } from "~/lib/enrollment-admin-api";
import type { CareOfferingSelectionMode } from "~/lib/enrollment-phase-api";

/**
 * In the optional selection mode a child may be approved without a booked
 * care offering; the school confirms that once (#2186). `bookedOfferings`
 * is undefined when the bookings could not be loaded: then nobody can tell,
 * so no confirmation is asked.
 */
export function approvalNeedsOfferingConfirm({
  status,
  careOfferingsEnabled,
  selectionMode,
  bookedOfferings,
}: {
  readonly status: DecisionStatus;
  readonly careOfferingsEnabled: boolean;
  readonly selectionMode: CareOfferingSelectionMode | undefined;
  readonly bookedOfferings: number | undefined;
}): boolean {
  return (
    status === "approved" &&
    careOfferingsEnabled &&
    selectionMode === "optional" &&
    bookedOfferings === 0
  );
}

export const APPROVAL_WITHOUT_OFFERING_COPY = {
  title: "Anmeldung bestätigen",
  confirmText: "Trotzdem bestätigen",
  message:
    "Für dieses Kind ist kein Betreuungsangebot gebucht. Das Kind wird trotzdem in die OGS aufgenommen.",
} as const;
