/**
 * Client of the invoices and amounts per family in the Träger portal
 * (#4691). A family is the recipient of its documents; its ID is the ID of
 * its first document. moto does not match payments: "open" is what the
 * invoices ask for after their Storno and Gutschriften.
 * Money stays whole cent. Backend int64 IDs leave here as strings.
 */

import type { BillingDocumentKind } from "./traeger-billing-api";
import { traegerRequest } from "./traeger-api";

/** Who pays an invoice. */
export type BillingPaymentKind = "family" | "cost_bearer" | "exempt";

export interface BillingPayment {
  readonly kind: BillingPaymentKind;
  /** The name the invoice copied; empty unless a cost bearer pays. */
  readonly costBearerName: string;
}

/** What an invoice still asks for. */
export type BillingInvoiceStatus =
  "open" | "partly_credited" | "credited" | "cancelled" | "nothing_due";

export interface BillingAmounts {
  readonly issuedCents: number;
  readonly cancelledCents: number;
  readonly creditedCents: number;
  readonly openCents: number;
}

export interface BillingFamily extends BillingAmounts {
  readonly id: string;
  readonly recipientFirstName: string;
  readonly recipientLastName: string;
  readonly recipientEmail: string;
  readonly debtorNumbers: readonly string[];
  readonly payments: readonly BillingPayment[];
  readonly invoiceCount: number;
  readonly openInvoiceCount: number;
  readonly documentCount: number;
  /** Calendar date "YYYY-MM-DD"; empty without a document. */
  readonly lastIssuedOn: string;
}

export interface BillingFamilyDocument {
  readonly id: string;
  readonly runId: string;
  readonly kind: BillingDocumentKind;
  readonly number: string;
  readonly issuedOn: string;
  readonly periodName: string;
  readonly serviceFrom: string;
  readonly serviceUntil: string;
  /** The number of the invoice a Storno or Gutschrift refers to. */
  readonly referenceNumber: string;
  readonly reason: string;
  readonly childNames: readonly string[];
  readonly debtorNumber: string;
  /** Negative for a Storno or Gutschrift. */
  readonly totalCents: number;
  /** Set for an invoice only. */
  readonly payment: BillingPayment | null;
  readonly status: BillingInvoiceStatus | null;
  readonly openCents: number;
}

export interface BillingFamilyDetail extends BillingFamily {
  /** Newest first. */
  readonly documents: readonly BillingFamilyDocument[];
}

interface BackendPayment {
  kind: BillingPaymentKind;
  cost_bearer_name?: string;
}

interface BackendFamily {
  id: number;
  recipient_first_name: string;
  recipient_last_name: string;
  recipient_email: string;
  debtor_numbers?: string[] | null;
  payments?: BackendPayment[] | null;
  invoice_count: number;
  open_invoice_count: number;
  document_count: number;
  last_issued_on?: string;
  issued_cents: number;
  cancelled_cents: number;
  credited_cents: number;
  open_cents: number;
}

interface BackendFamilyDocument {
  id: number;
  run_id: number;
  kind: BillingDocumentKind;
  number: string;
  issued_on: string;
  period_name: string;
  service_from: string;
  service_until: string;
  reference_number?: string;
  reason?: string;
  child_names?: string[] | null;
  debtor_number?: string;
  total_cents: number;
  payment?: BackendPayment | null;
  status?: BillingInvoiceStatus | "";
  open_cents: number;
}

interface BackendFamilyDetail extends BackendFamily {
  documents?: BackendFamilyDocument[] | null;
}

function paymentFromBackend(payment: BackendPayment): BillingPayment {
  return { kind: payment.kind, costBearerName: payment.cost_bearer_name ?? "" };
}

function familyFromBackend(family: BackendFamily): BillingFamily {
  return {
    id: String(family.id),
    recipientFirstName: family.recipient_first_name,
    recipientLastName: family.recipient_last_name,
    recipientEmail: family.recipient_email,
    debtorNumbers: family.debtor_numbers ?? [],
    payments: (family.payments ?? []).map(paymentFromBackend),
    invoiceCount: family.invoice_count,
    openInvoiceCount: family.open_invoice_count,
    documentCount: family.document_count,
    lastIssuedOn: family.last_issued_on ?? "",
    issuedCents: family.issued_cents,
    cancelledCents: family.cancelled_cents,
    creditedCents: family.credited_cents,
    openCents: family.open_cents,
  };
}

function documentFromBackend(
  document: BackendFamilyDocument,
): BillingFamilyDocument {
  return {
    id: String(document.id),
    runId: String(document.run_id),
    kind: document.kind,
    number: document.number,
    issuedOn: document.issued_on,
    periodName: document.period_name,
    serviceFrom: document.service_from,
    serviceUntil: document.service_until,
    referenceNumber: document.reference_number ?? "",
    reason: document.reason ?? "",
    childNames: document.child_names ?? [],
    debtorNumber: document.debtor_number ?? "",
    totalCents: document.total_cents,
    payment: document.payment ? paymentFromBackend(document.payment) : null,
    status: document.status || null,
    openCents: document.open_cents,
  };
}

const FAMILIES_PATH = "/api/traeger/billing-families";

/** The families with an issued document, by name. */
export async function fetchBillingFamilies(): Promise<BillingFamily[]> {
  const families = await traegerRequest<BackendFamily[] | null>(FAMILIES_PATH);
  return (families ?? []).map(familyFromBackend);
}

/** One family with every document, the newest first. */
export async function fetchBillingFamily(
  id: string,
): Promise<BillingFamilyDetail> {
  const family = await traegerRequest<BackendFamilyDetail>(
    `${FAMILIES_PATH}/${encodeURIComponent(id)}`,
  );
  return {
    ...familyFromBackend(family),
    documents: (family.documents ?? []).map(documentFromBackend),
  };
}
