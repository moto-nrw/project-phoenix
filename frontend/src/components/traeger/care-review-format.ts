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
