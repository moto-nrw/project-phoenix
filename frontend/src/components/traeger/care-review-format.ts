// Formate der Prüfphase eines Monatslaufs (#5916).

import { formatDate, parseISODate, toISODate } from "~/lib/date-helpers";
import { addMonths } from "./care-billing-format";

/**
 * The invoice dates a run of `month` ("YYYY-MM-01") accepts: from the first
 * day of the month before to the last day of the month after, as the
 * backend checks them.
 */
export function invoiceDateBounds(month: string): {
  min: string;
  max: string;
} {
  const after = parseISODate(addMonths(month, 2));
  after.setDate(0);
  return { min: addMonths(month, -1), max: toISODate(after) };
}

/** "30.10.2026", or what the release takes without a date. */
export function invoiceDateLabel(invoiceDate: string | null): string {
  return invoiceDate ? formatDate(invoiceDate) : "Tag der Freigabe";
}

/** "Alle 3 Schulen" or the names of the run's schools. */
export function runSchoolsLabel(
  schoolIds: readonly string[],
  schools: readonly { readonly id: string; readonly name: string }[],
): string {
  if (schoolIds.length === 0) return "–";
  if (schools.length > 1 && schoolIds.length === schools.length) {
    return `Alle ${schools.length} Schulen`;
  }
  const names = schoolIds.map(
    (id) => schools.find((school) => school.id === id)?.name ?? "Schule",
  );
  return names.join(", ");
}

/** An invoice of a run as the exclusion names it (#6346). */
interface ExclusionInvoice {
  readonly contractId: string;
  readonly service: "care" | "meal";
  readonly childFirstName: string;
  readonly totalCents: number;
  readonly mealPositions: readonly unknown[];
  readonly exclusion?: unknown;
}

/** What one invoice of a run bills: "Mensa-Rechnung", "Betreuung mit
 * Mensa" (the Mensa travels on it) or "Betreuung". */
export function invoiceKindLabel(invoice: ExclusionInvoice): string {
  if (invoice.service === "meal") return "Mensa-Rechnung";
  return invoice.mealPositions.length > 0 ? "Betreuung mit Mensa" : "Betreuung";
}

/** The other invoice of the same contract in the run, if there is one: the
 * Mensa invoice of its own of a care invoice, or the other way round. */
export function otherInvoiceOf<T extends ExclusionInvoice>(
  invoice: T,
  invoices: readonly T[],
): T | undefined {
  return invoices.find(
    (other) =>
      other.contractId === invoice.contractId &&
      other.service !== invoice.service,
  );
}

/** "Mensa-Rechnung ausschließen", "Betreuungsrechnung ausschließen" or,
 * when the contract has one invoice only, "Rechnung ausschließen". */
export function exclusionTitle<T extends ExclusionInvoice>(
  invoice: T,
  invoices: readonly T[],
): string {
  if (invoice.service === "meal") return "Mensa-Rechnung ausschließen";
  return otherInvoiceOf(invoice, invoices)
    ? "Betreuungsrechnung ausschließen"
    : "Rechnung ausschließen";
}

/**
 * What else the exclusion of this invoice affects, in one sentence, or null
 * when it affects nothing else. An exclusion leaves out exactly one invoice
 * (#6346): the other invoice of the contract stays, and Mensa that travels
 * on a care invoice goes with it.
 */
export function exclusionScopeText<T extends ExclusionInvoice>(
  invoice: T,
  invoices: readonly T[],
  formatEuro: (cents: number) => string,
): string | null {
  const other = otherInvoiceOf(invoice, invoices);
  if (other) {
    const name =
      other.service === "meal"
        ? "Die Mensa-Rechnung"
        : "Die Betreuungsrechnung";
    return other.exclusion
      ? `${name} von ${other.childFirstName} ist schon ausgeschlossen.`
      : `${name} von ${other.childFirstName} über ${formatEuro(other.totalCents)} bleibt im Lauf.`;
  }
  if (invoice.service === "care" && invoice.mealPositions.length > 0) {
    return "Die Mensa steht auf dieser Rechnung. Sie wird mit ausgeschlossen.";
  }
  return null;
}
