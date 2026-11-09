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
import { addDays } from "date-fns/addDays";
import { formatDate, parseISODate, toISODate } from "~/lib/date-helpers";

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

/** The due day all given hints share, or "" when they differ. */
function sharedDueDay(hints: readonly DebitHint[]): string {
  const days = new Set(hints.map((hint) => hint.item.payment?.dueOn ?? ""));
  return days.size === 1 ? ([...days][0] ?? "") : "";
}

/**
 * The first day a debit may come: the pre-notification days after the
 * invoice date (`notice_short` in the backend: due before issued + days).
 */
export function earliestDebitDay(issuedOn: string, days: number): string {
  return toISODate(addDays(parseISODate(issuedOn), days));
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
export function debitOpenPoints(
  hints: readonly DebitHint[],
  /** The first day the pre-notification allows, when the run knows it. */
  earliest?: string | null,
): string[] {
  const early = hints.filter((hint) => hint.dueTooSoon);
  const tooEarly = early.length;
  const halted = haltingHints(hints);
  const creditorMissing = halted.some(
    (hint) => hint.item.payment?.debitGap === "creditor_missing",
  );
  const day = sharedDueDay(early);
  return [
    tooEarly === 0
      ? null
      : earliest
        ? `${familiesLabel(tooEarly)}: ${day ? `Abbuchung am ${formatDate(day)} ist zu früh` : "Abbuchung zu früh"}, frühestens am ${formatDate(earliest)}.`
        : `${familiesLabel(tooEarly)}: Abbuchung früher als Ihre Vorabankündigung erlaubt.`,
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

/** The sentence of a debit that comes too early: how many families, which
 * day, and the rule (#4695). */
function tooEarlyText(
  count: number,
  day: string,
  prenotificationDays: number | null,
  issuedOn?: string | null,
): string {
  const when = day ? `der ${formatDate(day)}` : "die Abbuchung";
  const span =
    prenotificationDays === null
      ? ""
      : prenotificationDays === 1
        ? "1 Tag"
        : `${prenotificationDays} Tage`;
  const rule =
    prenotificationDays === null
      ? "Ihre Vorabankündigung braucht mehr Vorlauf."
      : issuedOn
        ? `Abbuchung frühestens am ${formatDate(earliestDebitDay(issuedOn, prenotificationDays))}, ${span} nach dem Rechnungsdatum.`
        : `Abbuchung frühestens ${span} nach dem Rechnungsdatum.`;
  return `Für ${familiesLabel(count)} ist ${when} zu früh. ${rule}`;
}

/**
 * The hint under the direct debit date: how many families the current day
 * debits sooner than the pre-notification allows, and the rule (#4695).
 */
export function tooEarlyHint(
  hints: readonly DebitHint[],
  prenotificationDays: number | null,
  /** The run's invoice date, when it is set: the hint names the first day. */
  issuedOn?: string | null,
): string | undefined {
  const early = hints.filter((hint) => hint.dueTooSoon);
  if (early.length === 0) return undefined;
  return tooEarlyText(
    early.length,
    sharedDueDay(early),
    prenotificationDays,
    issuedOn,
  );
}

/**
 * The same check for a day that is not saved yet, as the office types it:
 * the debits that follow the run's day (not a family's own pay day) come too
 * early when the day lies before the first day the pre-notification allows
 * (`notice_short` in the backend). Needs the invoice date to know that day.
 */
export function draftTooEarlyHint(
  items: readonly DebitItem[],
  day: string,
  prenotificationDays: number | null,
  issuedOn?: string | null,
): string | undefined {
  if (day === "" || prenotificationDays === null || !issuedOn) return undefined;
  if (day >= earliestDebitDay(issuedOn, prenotificationDays)) return undefined;
  const count = items.filter((item) => {
    const payment = item.payment;
    return (
      !item.excluded &&
      item.totalCents > 0 &&
      payment?.method === "direct_debit" &&
      !payment.debitGap &&
      Boolean(payment.mandate) &&
      payment.dueSource !== "family"
    );
  }).length;
  if (count === 0) return undefined;
  return tooEarlyText(count, day, prenotificationDays, issuedOn);
}
