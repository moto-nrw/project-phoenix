/**
 * Client of the invoices and amounts per family in the Träger portal
 * (#4691). A family is the recipient of its documents; its ID is the ID of
 * its first document. moto does not match payments: "open" is what the
 * invoices ask for after their Storno and Gutschriften.
 * Money stays whole cent. Backend int64 IDs leave here as strings.
 */

import type { BillingDocumentKind } from "./traeger-billing-api";
import {
  type BackendDebtorNumberFormat,
  type DebtorNumberFormat,
  debtorNumberFormatOf,
} from "./debtor-number-format";
import { traegerRequest } from "./traeger-api";
import type { PaymentMethod } from "./traeger-billing-terms-api";

/** Who pays an invoice. */
type BillingPaymentKind = "family" | "cost_bearer" | "exempt";

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
  /** The payment methods of its invoices (#4689), in form order. */
  readonly paymentMethods?: readonly PaymentMethod[];
  readonly invoiceCount: number;
  readonly openInvoiceCount: number;
  readonly documentCount: number;
  /** Calendar date "YYYY-MM-DD"; empty without a document. */
  readonly lastIssuedOn: string;
  /** The key of the family, the address of its registrations (#6692). */
  readonly familyEmail: string;
  /** The debtor number the family has at the Träger; "" for none. */
  readonly familyDebtorNumber: string;
  /** No care contract and no monthly invoice: known from the holiday care. */
  readonly holidayOnly: boolean;
  /** The family has an issued document, so it opens a page of its own. */
  readonly hasDocuments: boolean;
}

export interface BillingFamilyDocument {
  readonly id: string;
  /** Holiday care or monthly care (#4689); the run belongs to it. */
  readonly service?: "holiday" | "care";
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
  /** How and by when it is paid (#4689); "" and null on documents issued before. */
  readonly paymentMethod?: PaymentMethod | "";
  readonly dueOn?: string | null;
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
  payment_methods?: PaymentMethod[] | null;
  invoice_count: number;
  open_invoice_count: number;
  document_count: number;
  last_issued_on?: string;
  family_email?: string;
  family_debtor_number?: string;
  holiday_only?: boolean;
  issued_cents: number;
  cancelled_cents: number;
  credited_cents: number;
  open_cents: number;
}

interface BackendFamilyDocument {
  id: number;
  service?: string;
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
  payment_method?: PaymentMethod | "";
  due_on?: string | null;
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
    paymentMethods: family.payment_methods ?? [],
    invoiceCount: family.invoice_count,
    openInvoiceCount: family.open_invoice_count,
    documentCount: family.document_count,
    lastIssuedOn: family.last_issued_on ?? "",
    familyEmail: family.family_email ?? family.recipient_email,
    familyDebtorNumber: family.family_debtor_number ?? "",
    holidayOnly: family.holiday_only ?? false,
    hasDocuments: family.id !== 0,
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
    service: document.service === "care" ? "care" : "holiday",
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
    paymentMethod: document.payment_method ?? "",
    dueOn: document.due_on ?? null,
  };
}

const FAMILIES_PATH = "/api/traeger/billing-families";

/**
 * The families with an issued document or a number of their own, also the
 * families of the holiday care before their first invoice (#6692), by name.
 */
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

/**
 * Sets the debtor number of a family by its address (#6692), also of a
 * family without a care contract; "" removes it. Returns the number it has.
 */
export async function setFamilyDebtorNumber(
  familyEmail: string,
  debtorNumber: string,
): Promise<string> {
  const saved = await traegerRequest<{ debtor_number: string }>(
    `${FAMILIES_PATH}/debtor-number`,
    {
      method: "PUT",
      body: { family_email: familyEmail, debtor_number: debtorNumber },
    },
  );
  return saved.debtor_number;
}

/**
 * The format of the Träger's debtor numbers (#6675), so a field can name it
 * and check it before the backend does.
 */
export async function fetchDebtorNumberFormat(): Promise<DebtorNumberFormat> {
  return debtorNumberFormatOf(
    await traegerRequest<BackendDebtorNumberFormat | null>(
      `${FAMILIES_PATH}/debtor-number-format`,
    ),
  );
}
