/**
 * Client of who gets an invoice, how it is paid and by when (#4689): the
 * payment method, payment term and bundling per service, the deviation of a
 * family (another recipient, its own payment method or due day) and the due
 * dates of a calculated run per payment method.
 * Backend int64 IDs leave here as strings; calendar dates stay "YYYY-MM-DD".
 */

import { traegerRequest } from "./traeger-api";
import {
  type BackendInvoiceMandate,
  type DebitGap,
  type InvoiceMandate,
  invoiceMandateFromBackend,
} from "./traeger-sepa-api";

export type PaymentMethod = "transfer" | "direct_debit";
export type BillingService = "care" | "meal" | "holiday";
/** Who an invoice goes to; empty on an invoice issued before #4689. */
export type RecipientKind = "applicant" | "both" | "payer";
/** Where the due date of an invoice comes from. */
type DueSource = "family" | "run" | "term";

/** The payment methods in the order the forms show them. */
export const PAYMENT_METHODS: readonly PaymentMethod[] = [
  "transfer",
  "direct_debit",
];

/** The longest payment term in days. */
export const MAX_DUE_DAYS = 120;
/** The latest day of a month a family's invoices may be due on. */
export const MAX_DUE_DAY = 28;

export interface ServiceTerms {
  readonly service: BillingService;
  readonly paymentMethod: PaymentMethod;
  readonly dueDays: number;
  /** The service whose invoice carries this one's positions; "" = own invoice. */
  readonly bundleWith: string;
  /** False while the Träger keeps the defaults. */
  readonly custom: boolean;
  readonly updatedAt: string | null;
}

interface BillingServiceOption {
  readonly service: BillingService;
  /** The services whose invoice may carry this one; empty = always its own. */
  readonly bundleWith: readonly string[];
}

export interface BillingTerms {
  readonly services: readonly BillingServiceOption[];
  readonly terms: readonly ServiceTerms[];
}

export interface ServiceTermsInput {
  readonly paymentMethod: PaymentMethod;
  readonly dueDays: number;
  readonly bundleWith: string;
}

export interface PostalAddress {
  readonly street: string;
  readonly postalCode: string;
  readonly city: string;
}

/** The deviation of one family; the family is the e-mail of the contract party. */
export interface FamilyBilling {
  readonly id: string;
  readonly familyEmail: string;
  readonly contractId: string;
  readonly recipientKind: RecipientKind;
  readonly secondFirstName: string;
  readonly secondLastName: string;
  readonly payerFirstName: string;
  readonly payerLastName: string;
  readonly payerEmail: string;
  readonly payerAddress: PostalAddress | null;
  /** "" keeps the payment method of the service. */
  readonly paymentMethod: PaymentMethod | "";
  /** Day of the month 1–28; null keeps the date of the run. */
  readonly dueDay: number | null;
  readonly updatedAt: string;
}

export interface FamilyBillingInput {
  readonly contractId: string;
  readonly recipientKind: RecipientKind;
  readonly secondFirstName: string;
  readonly secondLastName: string;
  readonly payerFirstName: string;
  readonly payerLastName: string;
  readonly payerEmail: string;
  readonly payerAddress: PostalAddress | null;
  readonly paymentMethod: PaymentMethod | "";
  readonly dueDay: number | null;
}

/** The due date of a run for one payment method. */
export interface RunDueDate {
  readonly paymentMethod: PaymentMethod;
  /** Calendar date "YYYY-MM-DD". */
  readonly dueOn: string;
}

/** How and by when an invoice is paid, with where the date came from. */
export interface InvoicePayment {
  readonly method: PaymentMethod;
  readonly dueOn: string;
  readonly dueSource: DueSource;
  /** The SEPA mandate a direct debit is collected with (#4695). */
  readonly mandate?: InvoiceMandate | null;
  /** Why a family that pays by direct debit is not collected; null if it is. */
  readonly debitGap?: DebitGap | null;
}

interface BackendServiceTerms {
  service: BillingService;
  payment_method: PaymentMethod;
  due_days: number;
  bundle_with: string;
  custom: boolean;
  updated_at: string | null;
}

interface BackendBillingTerms {
  services: { service: BillingService; bundle_with: string[] | null }[] | null;
  terms: BackendServiceTerms[] | null;
}

interface BackendAddress {
  street: string;
  postal_code: string;
  city: string;
}

interface BackendFamilyBilling {
  id: number;
  family_email: string;
  contract_id: number;
  recipient_kind: RecipientKind;
  second_first_name: string;
  second_last_name: string;
  payer_first_name: string;
  payer_last_name: string;
  payer_email: string;
  payer_address: BackendAddress | null;
  payment_method: PaymentMethod | "";
  due_day: number | null;
  updated_at: string;
}

export interface BackendRunDueDate {
  payment_method: PaymentMethod;
  due_on: string;
}

export interface BackendInvoicePayment {
  method: PaymentMethod;
  due_on: string;
  due_source: DueSource;
  mandate?: BackendInvoiceMandate | null;
  debit_gap?: DebitGap | "";
}

function termsFromBackend(body: BackendBillingTerms): BillingTerms {
  return {
    services: (body.services ?? []).map((option) => ({
      service: option.service,
      bundleWith: option.bundle_with ?? [],
    })),
    terms: (body.terms ?? []).map((terms) => ({
      service: terms.service,
      paymentMethod: terms.payment_method,
      dueDays: terms.due_days,
      bundleWith: terms.bundle_with,
      custom: terms.custom,
      updatedAt: terms.updated_at,
    })),
  };
}

function familyFromBackend(family: BackendFamilyBilling): FamilyBilling {
  return {
    id: String(family.id),
    familyEmail: family.family_email,
    contractId: family.contract_id ? String(family.contract_id) : "",
    recipientKind: family.recipient_kind,
    secondFirstName: family.second_first_name,
    secondLastName: family.second_last_name,
    payerFirstName: family.payer_first_name,
    payerLastName: family.payer_last_name,
    payerEmail: family.payer_email,
    payerAddress: family.payer_address
      ? {
          street: family.payer_address.street,
          postalCode: family.payer_address.postal_code,
          city: family.payer_address.city,
        }
      : null,
    paymentMethod: family.payment_method,
    dueDay: family.due_day,
    updatedAt: family.updated_at,
  };
}

export function runDueDatesFromBackend(
  dates: readonly BackendRunDueDate[] | null | undefined,
): RunDueDate[] {
  return (dates ?? []).map((due) => ({
    paymentMethod: due.payment_method,
    dueOn: due.due_on,
  }));
}

export function invoicePaymentFromBackend(
  payment: BackendInvoicePayment | null | undefined,
): InvoicePayment | null {
  if (!payment) return null;
  return {
    method: payment.method,
    dueOn: payment.due_on,
    dueSource: payment.due_source,
    mandate: invoiceMandateFromBackend(payment.mandate),
    debitGap: payment.debit_gap ? payment.debit_gap : null,
  };
}

const TERMS_PATH = "/api/traeger/billing-terms";
const FAMILY_PATH = "/api/traeger/family-billing";
const DUE_DATES_PATH = "/api/traeger/billing-run-due-dates";

/** The terms of every service, the defaults where the Träger set none. */
export async function fetchBillingTerms(): Promise<BillingTerms> {
  return termsFromBackend(
    await traegerRequest<BackendBillingTerms>(TERMS_PATH),
  );
}

/** Sets the terms of one service; answers the terms of every service. */
export async function saveServiceTerms(
  service: BillingService,
  input: ServiceTermsInput,
): Promise<BillingTerms> {
  return termsFromBackend(
    await traegerRequest<BackendBillingTerms>(
      `${TERMS_PATH}/${encodeURIComponent(service)}`,
      {
        method: "PUT",
        body: {
          payment_method: input.paymentMethod,
          due_days: input.dueDays,
          bundle_with: input.bundleWith,
        },
      },
    ),
  );
}

/** The deviations of the families of the Träger. */
export async function fetchFamilyBillings(): Promise<FamilyBilling[]> {
  const families = await traegerRequest<BackendFamilyBilling[] | null>(
    FAMILY_PATH,
  );
  return (families ?? []).map(familyFromBackend);
}

/** The request body of a deviation; exported for its test. */
export function familyBillingBody(input: FamilyBillingInput) {
  const both = input.recipientKind === "both";
  const payer = input.recipientKind === "payer";
  return {
    contract_id: Number(input.contractId),
    recipient_kind: input.recipientKind,
    second_first_name: both ? input.secondFirstName.trim() : "",
    second_last_name: both ? input.secondLastName.trim() : "",
    payer_first_name: payer ? input.payerFirstName.trim() : "",
    payer_last_name: payer ? input.payerLastName.trim() : "",
    payer_email: payer ? input.payerEmail.trim() : "",
    payer_address:
      payer && input.payerAddress
        ? {
            street: input.payerAddress.street.trim(),
            postal_code: input.payerAddress.postalCode.trim(),
            city: input.payerAddress.city.trim(),
          }
        : null,
    payment_method: input.paymentMethod,
    due_day: input.dueDay,
  };
}

/**
 * Sets the deviation of the family of a contract. Null when nothing
 * deviates any more: then the family has no deviation.
 */
export async function saveFamilyBilling(
  input: FamilyBillingInput,
): Promise<FamilyBilling | null> {
  const body = await traegerRequest<{
    family: BackendFamilyBilling | null;
  } | null>(FAMILY_PATH, { method: "PUT", body: familyBillingBody(input) });
  return body?.family ? familyFromBackend(body.family) : null;
}

/**
 * Replaces the due dates of a calculated run; a payment method without a
 * date falls back to the payment term of the service.
 */
export async function saveRunDueDates(
  kind: "care" | "holiday",
  runId: string,
  dates: readonly RunDueDate[],
): Promise<RunDueDate[]> {
  const body = await traegerRequest<{
    due_dates: BackendRunDueDate[] | null;
  } | null>(`${DUE_DATES_PATH}/${kind}/${encodeURIComponent(runId)}`, {
    method: "PUT",
    body: {
      due_dates: dates.map((due) => ({
        payment_method: due.paymentMethod,
        due_on: due.dueOn,
      })),
    },
  });
  return runDueDatesFromBackend(body?.due_dates);
}
