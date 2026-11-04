import type { BillingChild } from "~/lib/traeger-billing-api";

// Restore the frozen amount, then apply the remaining proposals before the
// corrections of the run, as the backend preview does.
export function totalAfterRemovingCarriedCorrection(
  child: BillingChild,
  removedPositionId: string,
): number {
  let totalCents = child.totalCents;
  for (const correction of child.corrections) {
    totalCents -= correction.amountCents;
  }
  for (const carried of child.carriedCorrections) {
    totalCents -= carried.amountCents;
  }
  for (const carried of child.carriedCorrections) {
    if (carried.positionId !== removedPositionId) {
      totalCents += Math.max(carried.sourceAmountCents, -totalCents);
    }
  }
  for (const correction of child.corrections) {
    totalCents += correction.amountCents;
  }
  return totalCents;
}
