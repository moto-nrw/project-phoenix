/**
 * Display helpers of who gets an invoice, how it is paid and by when
 * (#4689): payment method, payment term, bundling, run due dates and the
 * deviation of a family.
 */

import { formatDate } from "~/lib/date-helpers";
import type {
  BillingService,
  FamilyBilling,
  InvoicePayment,
  PaymentMethod,
  RecipientKind,
  RunDueDate,
} from "~/lib/traeger-billing-terms-api";
import { PAYMENT_METHODS } from "~/lib/traeger-billing-terms-api";

export const PAYMENT_METHOD_LABELS: Record<PaymentMethod, string> = {
  transfer: "Überweisung",
  direct_debit: "Lastschrift",
};

export const BILLING_SERVICE_LABELS: Record<BillingService, string> = {
  care: "Monatsbeitrag Betreuung",
  meal: "Mittagessen",
  holiday: "Ferienbetreuung",
};

/** "Sofort", "1 Tag", "14 Tage". */
export function dueDaysLabel(days: number): string {
  if (days === 0) return "Sofort";
  return days === 1 ? "1 Tag" : `${days} Tage`;
}

/**
 * Which invoice carries a service: its own, or the invoice of the service
 * it travels with. `options` are the services it may travel with; `carries`
 * tells that another service may travel on its invoice. A service with
 * neither (the holiday care) always has its own invoice.
 */
export function bundleLabel(
  bundleWith: string,
  options: readonly string[],
  carries = false,
): string {
  if (bundleWith === "care") return "Auf der Rechnung der Betreuung";
  if (options.length === 0 && !carries) return "Immer eigene Rechnung";
  return "Eigene Rechnung";
}

/** The recipient kind as a short phrase; null for the contract party. */
export function recipientKindLabel(
  kind: RecipientKind | "" | undefined,
): string | null {
  if (kind === "both") return "Beide Eltern";
  if (kind === "payer") return "Abweichender Zahler";
  return null;
}

/**
 * The name line of an invoice's recipient: "An Max Muster", for both
 * parents "An Max Muster und Erika Muster".
 */
export function recipientLine(recipient: {
  readonly firstName: string;
  readonly lastName: string;
  readonly kind?: RecipientKind | "";
  readonly secondName?: string;
}): string {
  const name = `${recipient.firstName} ${recipient.lastName}`.trim();
  if (!name) return "Ohne Empfänger";
  const second = recipient.secondName?.trim();
  if (recipient.kind === "both" && second) return `An ${name} und ${second}`;
  return `An ${name}`;
}

/** "fällig am 15.12.2026". */
export function dueOnText(payment: InvoicePayment): string {
  return `fällig am ${formatDate(payment.dueOn)}`;
}

/**
 * The hint beside a due date that the family set, not the run or the
 * payment term; null otherwise.
 */
export function dueSourceHint(payment: InvoicePayment): string | null {
  return payment.dueSource === "family" ? "Zahltag der Familie" : null;
}

/** "Lastschrift am 01.12.2026 · Überweisung am 15.12.2026", or null. */
export function runDueDatesText(dates: readonly RunDueDate[]): string | null {
  const ordered = PAYMENT_METHODS.flatMap((method) =>
    dates.filter((due) => due.paymentMethod === method && due.dueOn),
  );
  if (ordered.length === 0) return null;
  return ordered
    .map(
      (due) =>
        `${PAYMENT_METHOD_LABELS[due.paymentMethod]} am ${formatDate(due.dueOn)}`,
    )
    .join(" · ");
}

/** The payment methods of a family as one line, in form order. */
export function paymentMethodsText(
  methods: readonly string[] | undefined,
): string | null {
  const known = PAYMENT_METHODS.filter((method) =>
    (methods ?? []).includes(method),
  );
  if (known.length === 0) return null;
  return known.map((method) => PAYMENT_METHOD_LABELS[method]).join(", ");
}

/**
 * The deviation that applies to an invoice: the one saved for its
 * contract, else the one of its family. The family is the e-mail of the
 * contract party; a payer deviation sends the invoice to the payer, so the
 * invoice then names the payer's address instead.
 */
export function findFamilyBilling(
  families: readonly FamilyBilling[],
  invoice: {
    readonly contractId: string;
    readonly recipientEmail: string;
    readonly recipientKind?: RecipientKind | "";
  },
): FamilyBilling | null {
  const byContract = families.find(
    (family) =>
      family.contractId !== "" && family.contractId === invoice.contractId,
  );
  if (byContract) return byContract;
  const email = invoice.recipientEmail.trim().toLowerCase();
  if (!email) return null;
  if (invoice.recipientKind === "payer") {
    return (
      families.find(
        (family) =>
          family.recipientKind === "payer" &&
          family.payerEmail.trim().toLowerCase() === email,
      ) ?? null
    );
  }
  return families.find((family) => family.familyEmail === email) ?? null;
}
