/** Display helpers of the billing runs in the Träger portal (#4668). */

import type {
  BillingDocument,
  BillingDocumentKind,
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
  if (position.kind === "credit") return `Gutschrift: ${position.note}`;
  return contributionLineLabel({ ...position, kind: position.kind });
}

/** What a document of a run is (#4690). */
export const BILLING_DOCUMENT_KIND_LABELS: Record<BillingDocumentKind, string> =
  {
    invoice: "Rechnung",
    cancellation: "Storno",
    credit: "Gutschrift",
  };

/** An issued invoice with a Storno is no longer valid. */
export function isCancelled(document: BillingDocument): boolean {
  return document.history.some((link) => link.kind === "cancellation");
}

/** An issued invoice with a Gutschrift; a Storno is then refused. */
export function isCredited(document: BillingDocument): boolean {
  return document.history.some((link) => link.kind === "credit");
}

/**
 * The line under a document number: what it refers to. A Storno or
 * Gutschrift names its invoice, a new invoice the one it replaces.
 */
export function billingReferenceText(document: BillingDocument): string {
  if (!document.referenceNumber) return "";
  return document.kind === "invoice"
    ? `Ersetzt ${document.referenceNumber}`
    : `Zu ${document.referenceNumber}`;
}
