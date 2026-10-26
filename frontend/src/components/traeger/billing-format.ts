/** Display helpers of the billing runs in the Träger portal (#4668). */

import type {
  BillingPosition,
  BillingRunStatus,
} from "~/lib/traeger-billing-api";
import { contributionLineLabel } from "./contribution-format";

export const BILLING_RUN_STATUS_LABELS: Record<BillingRunStatus, string> = {
  calculated: "Vorschau",
  released: "Freigegeben",
};

export const BILLING_RUN_STATUS_TONES: Record<
  BillingRunStatus,
  "orange" | "green"
> = {
  calculated: "orange",
  released: "green",
};

/** What one position of an invoice stands for. */
export function billingPositionLabel(position: BillingPosition): string {
  if (position.kind === "correction") return `Korrektur: ${position.note}`;
  return contributionLineLabel({ ...position, kind: position.kind });
}
