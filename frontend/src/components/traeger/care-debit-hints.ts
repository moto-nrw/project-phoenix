// The SEPA hints of a run preview (#4695) as data: which invoices cannot be
// collected as planned and why. Kept apart from the cards so the run pages
// can count the hints for the release question without loading the cards.

import type { BillingDocument } from "~/lib/traeger-billing-api";
import type { InvoicePayment } from "~/lib/traeger-billing-terms-api";
import type { CareInvoice } from "~/lib/traeger-care-billing-api";
import { formatDate } from "~/lib/date-helpers";

function childName(invoice: CareInvoice): string {
  return `${invoice.childFirstName} ${invoice.childLastName}`.trim();
}

/** Why a family that pays by direct debit is not collected (#4695). Kept
 * here so the run page does not load the mandate forms' module. */
const DEBIT_GAP_LABELS = {
  mandate_missing: "Kein gültiges Mandat",
  creditor_missing: "Gläubiger-ID fehlt",
  creditor_changed: "Mandat für eine frühere Gläubiger-ID",
} as const;

/** A family's invoice as the hints need it, from a care or a holiday run. */
export interface DebitItem {
  readonly key: string;
  readonly name: string;
  /** Names the family for screen readers. */
  readonly family: string;
  readonly detail?: string;
  readonly totalCents: number;
  readonly payment?: InvoicePayment | null;
  /** Where its mandate is managed. */
  readonly href: string;
  readonly excluded?: boolean;
}

export interface DebitHint {
  readonly item: DebitItem;
  readonly text: string;
  /** The debit comes too early: the fix is a later due date, not the mandate. */
  readonly dueTooSoon?: boolean;
}

/** The SEPA hints of a preview (#4695): direct debits without a valid
 * mandate or creditor identifier, and debits sooner than the
 * pre-notification days of the Träger. */
export function debitHints(items: readonly DebitItem[]): DebitHint[] {
  const hints: DebitHint[] = [];
  for (const item of items) {
    // An excluded invoice is not issued; one of 0 € collects nothing.
    if (item.excluded || item.totalCents <= 0) continue;
    const payment = item.payment;
    if (payment?.debitGap) {
      hints.push({
        item,
        text:
          payment.method === "transfer"
            ? `${DEBIT_GAP_LABELS[payment.debitGap]}. Geht als Überweisung, fällig am ${formatDate(payment.dueOn)}.`
            : `${DEBIT_GAP_LABELS[payment.debitGap]}. Die Freigabe ist angehalten.`,
      });
    } else if (payment?.mandate?.noticeShort) {
      hints.push({
        item,
        text: `Abbuchung schon am ${formatDate(payment.dueOn)}. Das ist früher als Ihre Vorabankündigung erlaubt.`,
        dueTooSoon: true,
      });
    }
  }
  return hints;
}

/** The invoices of a care run as debit items. */
export function careDebitItems(invoices: readonly CareInvoice[]): DebitItem[] {
  return invoices.map((invoice) => ({
    key: `${invoice.contractId}-${invoice.service}`,
    name: childName(invoice),
    family: `Familie von ${childName(invoice)}`,
    detail: invoice.schoolName,
    totalCents: invoice.totalCents,
    payment: invoice.payment,
    href: `/traeger/abrechnung/lastschrift/${invoice.contractId}`,
    excluded: Boolean(invoice.exclusion),
  }));
}

/** The invoices of a holiday run as debit items: a holiday invoice has no
 * contract, so its way leads to the mandate overview. */
export function holidayDebitItems(
  documents: readonly BillingDocument[],
): DebitItem[] {
  return documents
    .filter((document) => document.kind === "invoice")
    .map((document) => {
      const name =
        `${document.recipientFirstName} ${document.recipientLastName}`.trim();
      return {
        key: document.registrationId,
        name,
        family: name,
        totalCents: document.totalCents,
        payment: document.payment,
        href: "/traeger/abrechnung/lastschrift",
      };
    });
}
