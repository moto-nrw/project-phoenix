/**
 * Display helpers of the care contracts in the Träger portal (#4678). The
 * texts of term and notice match the table "Vertragsangaben" of the contract
 * PDF, so the screen says what the parents read.
 */

import type { StatusBadgeTone } from "~/components/ui/status-badge";
import { formatDate } from "~/lib/date-helpers";
import type {
  CareContractStatus,
  ContractMailState,
  ContractNoticeTo,
  ContractTermKind,
  ContractTerms,
  CreateSkipReason,
  SendSkipReason,
  Weekday,
} from "~/lib/traeger-care-contracts-api";

export const CONTRACT_STATUS_LABELS: Record<CareContractStatus, string> = {
  entwurf: "Entwurf",
  versendet: "Versendet",
  unterschrieben: "Unterschrieben",
  aktiv: "Aktiv",
  beendet: "Beendet",
};

export const CONTRACT_STATUS_TONES: Record<
  CareContractStatus,
  StatusBadgeTone
> = {
  entwurf: "gray",
  versendet: "blue",
  unterschrieben: "purple",
  aktiv: "green",
  beendet: "outline",
};

/** The order of the status filter: what the office works on first. */
export const CONTRACT_STATUS_ORDER: readonly CareContractStatus[] = [
  "entwurf",
  "versendet",
  "unterschrieben",
  "aktiv",
  "beendet",
];

export const MAIL_STATE_LABELS: Record<ContractMailState, string> = {
  queued: "Wird gesendet",
  sent: "Gesendet",
  failed: "Nicht angekommen",
  unknown: "Ergebnis unbekannt",
};

export const MAIL_STATE_TONES: Record<ContractMailState, StatusBadgeTone> = {
  queued: "orange",
  sent: "green",
  failed: "red",
  unknown: "gray",
};

const WEEKDAY_SHORT: Record<Weekday, string> = {
  mon: "Mo",
  tue: "Di",
  wed: "Mi",
  thu: "Do",
  fri: "Fr",
  sat: "Sa",
  sun: "So",
};

const WEEKDAY_ORDER: readonly Weekday[] = [
  "mon",
  "tue",
  "wed",
  "thu",
  "fri",
  "sat",
  "sun",
];

/** The booked weekdays, short and in weekday order: „Mo, Di, Mi“. */
export function formatWeekdays(days: readonly Weekday[]): string {
  const shown = WEEKDAY_ORDER.filter((day) => days.includes(day)).map(
    (day) => WEEKDAY_SHORT[day],
  );
  return shown.length > 0 ? shown.join(", ") : "–";
}

export const WEEKDAY_CHIPS: readonly { day: Weekday; label: string }[] =
  WEEKDAY_ORDER.slice(0, 5).map((day) => ({ day, label: WEEKDAY_SHORT[day] }));

export const TERM_KIND_LABELS: Record<ContractTermKind, string> = {
  school_year: "Bis zum Ende des Schuljahres",
  open_ended: "Unbefristet",
  fixed_months: "Feste Zahl an Monaten",
};

export const NOTICE_TO_LABELS: Record<ContractNoticeTo, string> = {
  month_end: "zum Monatsende",
  half_year: "zum Halbjahr (31. Januar oder 31. Juli)",
  school_year_end: "zum Ende des Schuljahres",
};

/** The term as the contract prints it. */
export function formatTerm(
  terms: Pick<ContractTerms, "termKind" | "termMonths">,
  endsOn?: string | null,
): string {
  let text: string;
  switch (terms.termKind) {
    case "school_year":
      text = "Bis zum Ende des Schuljahres";
      break;
    case "fixed_months":
      text =
        terms.termMonths === 1
          ? "1 Monat"
          : `${terms.termMonths ?? "–"} Monate`;
      break;
    default:
      text = "Unbefristet";
  }
  return endsOn ? `${text}, endet am ${formatDate(endsOn)}` : text;
}

/** The notice period as the contract prints it. */
export function formatNotice(
  terms: Pick<ContractTerms, "noticeMonths" | "noticeTo">,
): string {
  const to = NOTICE_TO_LABELS[terms.noticeTo];
  if (terms.noticeMonths === 0) return `Ohne Frist ${to}`;
  if (terms.noticeMonths === 1) return `1 Monat Frist ${to}`;
  return `${terms.noticeMonths} Monate Frist ${to}`;
}

export const CREATE_SKIP_LABELS: Record<CreateSkipReason, string> = {
  template_missing: "Vertragsbedingungen fehlen",
  contract_exists: "Hat schon einen Vertrag",
  application_not_found: "Anmeldung nicht mehr angenommen",
};

export const SEND_SKIP_LABELS: Record<SendSkipReason, string> = {
  recipient_missing: "keine E-Mail-Adresse",
  mail_pending: "die letzte Mail wird noch gesendet",
  not_sendable: "lässt sich nicht mehr senden",
};

/** The full name of the child. */
export function childName(contract: {
  readonly childFirstName: string;
  readonly childLastName: string;
}): string {
  return `${contract.childFirstName} ${contract.childLastName}`.trim();
}
