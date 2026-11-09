// Wording of the SEPA direct debit file (#4697): the sequence types as the
// office reads them, why a debit stays out of the file, and the days the
// bank collects. Kept apart from the mandate forms so a run page loads only
// these few lines.

import type { StatusBadgeTone } from "~/components/ui/status-badge";
import { formatDate } from "~/lib/date-helpers";
import type {
  SepaDebitGroup,
  SepaDebitLeft,
  SepaSequenceType,
} from "~/lib/traeger-sepa-debits-api";

export const SEPA_SEQUENCE_BADGES: Record<
  SepaSequenceType,
  { readonly label: string; readonly tone: StatusBadgeTone }
> = {
  FRST: { label: "Erste Lastschrift", tone: "blue" },
  RCUR: { label: "Folgelastschrift", tone: "gray" },
  OOFF: { label: "Einmalig", tone: "purple" },
};

export const SEPA_DEBIT_LEFT_LABELS: Record<SepaDebitLeft, string> = {
  mandate_missing:
    "Die Familie hat kein gültiges Lastschriftmandat. Erfassen Sie das Mandat, oder die Familie überweist die Rate.",
  one_off_used:
    "Dieses Einmalmandat wurde bereits genutzt. Die Familie braucht ein neues Mandat.",
  one_off_multiple:
    "Mehrere Rechnungen nutzen dieses Einmalmandat. Klären Sie mit der Familie, wie sie zahlt.",
  cancelled: "Die Rechnung ist storniert oder gutgeschrieben.",
  revoked: "Das Mandat ist widerrufen.",
  expired:
    "Das Mandat ist verfallen: 36 Monate lang wurde nicht abgebucht. Die Familie braucht ein neues Mandat.",
  creditor_changed:
    "Das Mandat gilt für eine frühere Gläubiger-ID. Die Familie braucht ein neues Mandat.",
};

/** „1 Lastschrift“, „3 Lastschriften“. */
export function debitCountLabel(count: number): string {
  return count === 1 ? "1 Lastschrift" : `${count} Lastschriften`;
}

/** The days the bank collects: one date, or the first and how many more. */
export function collectDatesText(groups: readonly SepaDebitGroup[]): string {
  const dates = [...new Set(groups.map((group) => group.collectOn))].sort();
  const [first] = dates;
  if (!first) return "–";
  if (dates.length === 1) return formatDate(first);
  return `${formatDate(first)} und ${dates.length - 1} weitere`;
}

/** The invoice number, or for an installment of a Jahresbescheid the
 * notice and its month: „MB-2027-0041 · Rate 01/2027“. */
export function debitDocumentLabel(debit: {
  readonly invoiceNumber: string;
  readonly noticeMonth: string | null;
}): string {
  if (!debit.noticeMonth) return debit.invoiceNumber;
  const [year, month] = debit.noticeMonth.split("-");
  return `${debit.invoiceNumber} · Rate ${month}/${year}`;
}
