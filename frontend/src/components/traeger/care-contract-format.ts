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
  ContractService,
  ContractTerms,
  SendSkipReason,
  TemplateIssue,
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

const WEEKDAY_LONG: Record<Weekday, string> = {
  mon: "Montag",
  tue: "Dienstag",
  wed: "Mittwoch",
  thu: "Donnerstag",
  fri: "Freitag",
  sat: "Samstag",
  sun: "Sonntag",
};

/** One weekday of a contract with the modules booked on it. */
export interface ServiceDay {
  readonly weekday: Weekday;
  readonly label: string;
  readonly services: readonly ContractService[];
}

/**
 * The weekdays of a contract as the PDF prints them: Monday to Friday
 * always, Saturday and Sunday only when booked. A day without services has
 * no care.
 */
export function serviceDays(
  services: readonly ContractService[],
): ServiceDay[] {
  return WEEKDAY_ORDER.map((weekday) => ({
    weekday,
    label: WEEKDAY_LONG[weekday],
    services: services.filter((service) => service.weekday === weekday),
  })).filter((day, index) => index < 5 || day.services.length > 0);
}

/** One module of a day: „Kurzbetreuung bis 14:00 Uhr“. */
export function formatService(service: ContractService): string {
  return service.endsAt
    ? `${service.moduleName} bis ${service.endsAt} Uhr`
    : service.moduleName;
}

/** The modules of a contract once each: „Frühbetreuung, Ganztag“. */
export function formatModules(names: readonly string[]): string {
  return names.length > 0 ? names.join(", ") : "–";
}

/** The module names of the services once each, in weekday order. */
export function moduleNames(services: readonly ContractService[]): string[] {
  return [...new Set(services.map((service) => service.moduleName))];
}

/**
 * The booked days in short, with the end time of each module:
 * „Mo bis 08:00 und 14:00 · Di bis 16:30“.
 */
export function formatServiceDays(
  services: readonly ContractService[],
): string {
  const days = serviceDays(services).filter((day) => day.services.length > 0);
  if (days.length === 0) return "–";
  return days
    .map((day) => {
      const ends = day.services.map(
        (service) => service.endsAt || service.moduleName,
      );
      const timed = day.services.every((service) => service.endsAt);
      return `${WEEKDAY_SHORT[day.weekday]} ${timed ? "bis " : ""}${ends.join(" und ")}`;
    })
    .join(" · ");
}

/** Why a child gets no draft yet, as the create dialog says it. */
export const TEMPLATE_ISSUE_LABELS: Record<TemplateIssue, string> = {
  template_missing: "Vertragsbedingungen fehlen",
  templates_differ: "Bedingungen der Module weichen ab",
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
