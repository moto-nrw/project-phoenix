import type { BillingChild } from "~/lib/traeger-billing-api";

// Proposals apply in order and each may lower the amount only to zero. Rebuild
// that sequence without one proposal to match the resulting backend preview.
export function totalAfterRemovingCarriedCorrection(
  child: BillingChild,
  removedPositionId: string,
): number {
  let totalCents = child.totalCents;
  for (const carried of child.carriedCorrections) {
    totalCents -= carried.amountCents;
  }
  for (const carried of child.carriedCorrections) {
    if (carried.positionId !== removedPositionId) {
      totalCents += Math.max(carried.sourceAmountCents, -totalCents);
    }
  }
  return totalCents;
}
