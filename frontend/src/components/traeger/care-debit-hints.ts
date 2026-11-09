// The SEPA hints of a run preview (#4695) as data: which invoices cannot be
// collected as planned and why. Kept apart from the cards so the run pages
// can count the hints for the release question without loading the cards.

import type { BillingDocument } from "~/lib/traeger-billing-api";
import type {
  InvoicePayment,
  PaymentMethod,
} from "~/lib/traeger-billing-terms-api";
import type { CareInvoice } from "~/lib/traeger-care-billing-api";
import { familiesLabel } from "~/components/traeger/release-open-points";
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

/** Debits without mandate or creditor identifier: the release refuses a
 * run with one of them (MissingMandateBlock). A family that falls back to a
 * transfer pays all the same. */
function haltingHints(hints: readonly DebitHint[]): DebitHint[] {
  return hints.filter(
    (hint) => !hint.dueTooSoon && hint.item.payment?.method !== "transfer",
  );
}

/** Whether the release would be refused for a debit without mandate. */
export function debitsHaltRelease(hints: readonly DebitHint[]): boolean {
  return haltingHints(hints).length > 0;
}

/**
 * The SEPA lines of the release question: a debit that comes too early, and
 * a debit without mandate or creditor identifier, which the release refuses.
 * A family that falls back to a transfer pays all the same: no open point.
 */
export function debitOpenPoints(hints: readonly DebitHint[]): string[] {
  const tooEarly = hints.filter((hint) => hint.dueTooSoon).length;
  const halted = haltingHints(hints);
  const creditorMissing = halted.some(
    (hint) => hint.item.payment?.debitGap === "creditor_missing",
  );
  return [
    tooEarly > 0
      ? `${familiesLabel(tooEarly)}: Abbuchung früher als Ihre Vorabankündigung erlaubt.`
      : null,
    halted.length === 0
      ? null
      : creditorMissing
        ? "Gläubiger-ID fehlt: Die Freigabe ist angehalten."
        : `${familiesLabel(halted.length)} ohne gültiges Mandat: Die Freigabe ist angehalten.`,
  ].filter((point): point is string => point !== null);
}

/**
 * The due date per payment method that applies without a date of the run:
 * the payment term of the service, when every invoice of the method shares
 * it. A family with its own pay day does not count.
 */
export function termDueDates(
  items: readonly DebitItem[],
): Partial<Record<PaymentMethod, string>> {
  const dates = new Map<PaymentMethod, Set<string>>();
  for (const item of items) {
    const payment = item.payment;
    if (item.excluded || item.totalCents <= 0 || payment?.dueSource !== "term")
      continue;
    const known = dates.get(payment.method) ?? new Set<string>();
    known.add(payment.dueOn);
    dates.set(payment.method, known);
  }
  const uniform: Partial<Record<PaymentMethod, string>> = {};
  for (const [method, known] of dates) {
    if (known.size === 1) uniform[method] = [...known][0];
  }
  return uniform;
}

/**
 * The hint under the direct debit date: how many families the current day
 * debits sooner than the pre-notification allows, and the rule (#4695).
 */
export function tooEarlyHint(
  hints: readonly DebitHint[],
  prenotificationDays: number | null,
): string | undefined {
  const early = hints.filter((hint) => hint.dueTooSoon);
  if (early.length === 0) return undefined;
  const days = new Set(early.map((hint) => hint.item.payment?.dueOn ?? ""));
  const day = days.size === 1 ? [...days][0] : "";
  const when = day ? `der ${formatDate(day)}` : "die Abbuchung";
  const rule =
    prenotificationDays === null
      ? "Ihre Vorabankündigung braucht mehr Vorlauf."
      : `Abbuchung frühestens ${prenotificationDays === 1 ? "1 Tag" : `${prenotificationDays} Tage`} nach dem Rechnungsdatum.`;
  return `Für ${familiesLabel(early.length)} ist ${when} zu früh. ${rule}`;
}
