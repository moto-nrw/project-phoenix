/** Display helpers of the billing runs in the Träger portal (#4668). */

import type {
  BillingDocument,
  BillingDocumentKind,
  BillingPosition,
  BillingRunDetail,
  BillingRunStatus,
} from "~/lib/traeger-billing-api";
import { contributionLineLabel } from "./contribution-format";

export const BILLING_RUN_STATUS_LABELS: Record<BillingRunStatus, string> = {
  calculated: "Vorschau",
  released: "Freigegeben",
  discarded: "Verworfen",
};

export const BILLING_RUN_STATUS_TONES: Record<
  BillingRunStatus,
  "orange" | "green" | "gray"
> = {
  calculated: "orange",
  released: "green",
  discarded: "gray",
};

/**
 * An open run with nothing to issue and nothing to settle (#6628): no
 * invoice, no cancelled invoice to issue again and no confirmed
 * registration waiting for its contribution. Only such a run can be
 * discarded; the backend checks the same under the lock of the release.
 */
export function isEmptyBillingRun(run: BillingRunDetail): boolean {
  return (
    run.status === "calculated" &&
    run.documents.length === 0 &&
    run.unbilled === 0
  );
}

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

/**
 * An example of the invoice numbers a run will get, before the release gives
 * the real ones: the year filled in and the running number at the pattern's
 * width („FE-2026-0001“), so its digits show. Callers name it an example.
 * The pattern's tokens are the backend's: {JJJJ} or {JJ}, and {N} to
 * {NNNNNNNN}.
 */
export function numberPatternPreview(pattern: string, issueOn: string): string {
  const year = issueOn.slice(0, 4);
  return pattern
    .replaceAll("{JJJJ}", year)
    .replaceAll("{JJ}", year.slice(2))
    .replace(/\{(N{1,8})\}/g, (_token, digits: string) =>
      "1".padStart(digits.length, "0"),
    );
}
