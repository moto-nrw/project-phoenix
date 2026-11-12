/** Display helpers of the Jahresbescheide (#6219). */

import type { StatusBadgeTone } from "~/components/ui/status-badge";
import { formatDate } from "~/lib/date-helpers";
import type {
  CareNotice,
  CareNoticeKind,
  CareNoticeRunDetail,
  CareNoticeRunStatus,
} from "~/lib/traeger-care-notices-api";
import { formatEuro } from "./contribution-format";

export const NOTICE_RUN_STATUS_LABELS: Record<CareNoticeRunStatus, string> = {
  calculated: "Vorschau",
  released: "Ausgestellt",
  discarded: "Verworfen",
};

export const NOTICE_RUN_STATUS_TONES: Record<
  CareNoticeRunStatus,
  StatusBadgeTone
> = {
  calculated: "orange",
  released: "green",
  discarded: "gray",
};

export const NOTICE_KIND_LABELS: Record<CareNoticeKind, string> = {
  initial: "Bescheid",
  change: "Änderungsbescheid",
};

export const NOTICE_KIND_TONES: Record<CareNoticeKind, StatusBadgeTone> = {
  initial: "blue",
  change: "purple",
};

/** "2025-09-01", "2025-12-31" → "01.09.–31.12.2025", as MK-007 prints it. */
export function noticePeriod(from: string, until: string): string {
  const [fromYear, fromMonth, fromDay] = from.split("-");
  const [untilYear, untilMonth, untilDay] = until.split("-");
  if (fromYear === untilYear) {
    return `${fromDay}.${fromMonth}.–${untilDay}.${untilMonth}.${untilYear}`;
  }
  return `${formatDate(from)}–${formatDate(until)}`;
}

/** A signed euro amount for a Zu- or Abgang: "+48,00 €", "−24,00 €", "0,00 €". */
export function formatDelta(cents: number): string {
  if (cents === 0) return formatEuro(0);
  return `${cents > 0 ? "+" : "−"}${formatEuro(Math.abs(cents))}`;
}

/** "1 Bescheid", "3 Bescheide". */
export function noticeCountLabel(count: number): string {
  return count === 1 ? "1 Bescheid" : `${count} Bescheide`;
}

/**
 * The monthly rate of a notice as one figure when every month costs the
 * same ("32,00 € im Monat"), else the span ("32,00 € bis 40,00 € im Monat").
 */
export function noticeRateLabel(notice: CareNotice): string {
  const amounts = notice.installments.map((rate) => rate.amountCents);
  if (amounts.length === 0) return "–";
  const low = Math.min(...amounts);
  const high = Math.max(...amounts);
  return low === high
    ? `${formatEuro(low)} im Monat`
    : `${formatEuro(low)} bis ${formatEuro(high)} im Monat`;
}

/** "4 Raten" or "1 Rate". */
export function rateCountLabel(count: number): string {
  return count === 1 ? "1 Rate" : `${count} Raten`;
}

/**
 * An open notice run that concerns no contract at all (#6626): no notice, no
 * booking without a price, no child held back, no notice that still holds and
 * no month a monthly invoice bills. Such a run is discarded. A run whose
 * contracts all keep their notice is closed instead, as before: closing it
 * records that nothing changed. The server decides again under the lock of
 * the release.
 */
export function isEmptyNoticeRun(run: CareNoticeRunDetail): boolean {
  return (
    run.status === "calculated" &&
    run.previewCount === 0 &&
    run.unpricedCount === 0 &&
    run.duplicateCount === 0 &&
    run.unchanged === 0 &&
    run.billed === 0
  );
}
