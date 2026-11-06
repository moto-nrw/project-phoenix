/**
 * Display helpers of the care contracts in the Träger portal (#4678). The
 * texts of term and notice match the table "Vertragsangaben" of the contract
 * PDF, so the screen says what the parents read.
 */

import type { StatusBadgeTone } from "~/components/ui/status-badge";
import {
  formatBerlinDate,
  formatBerlinTime,
  formatDate,
} from "~/lib/date-helpers";
import type {
  CareContractStatus,
  ConclusionMethod,
  ConclusionMode,
  ContractConclusion,
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
 * always, Saturday and Sunday only when booked or named in `include` (a
 * weekend day another version books). A day without services has no care.
 */
export function serviceDays(
  services: readonly ContractService[],
  include: readonly Weekday[] = [],
): ServiceDay[] {
  return WEEKDAY_ORDER.map((weekday) => ({
    weekday,
    label: WEEKDAY_LONG[weekday],
    services: services.filter((service) => service.weekday === weekday),
  })).filter(
    (day, index) =>
      index < 5 || day.services.length > 0 || include.includes(day.weekday),
  );
}

/** One module of a day: „Kurzbetreuung bis 14:00 Uhr“. */
export function formatService(service: ContractService): string {
  const text = service.endsAt
    ? `${service.moduleName} bis ${service.endsAt} Uhr`
    : service.moduleName;
  return service.lastDay
    ? `${text}, gebucht bis ${formatDate(service.lastDay)}`
    : text;
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
  conclusion_automatic: "Entsteht bei der Annahme",
  acceptance_missing: "Zustimmung zum Vertrag fehlt",
  acceptance_changed: "Bedingungen nach der Anmeldung geändert",
};

/**
 * The reason in one plain sentence, with the next step, where the label
 * alone does not tell what happens (#4680).
 */
export const TEMPLATE_ISSUE_REASONS: Partial<Record<TemplateIssue, string>> = {
  conclusion_automatic: "Der Vertrag entsteht in wenigen Minuten von selbst.",
  acceptance_missing:
    "Die Eltern haben in der Anmeldung den Vertragsbedingungen nicht zugestimmt. Es entsteht kein Vertrag. Zum Senden stellen Sie die Schule auf „Später verschicken“.",
  acceptance_changed:
    "Sie haben die Bedingungen nach der Anmeldung geändert. Die Eltern haben ihnen nicht zugestimmt. Es entsteht kein Vertrag. Zum Senden stellen Sie die Schule auf „Später verschicken“.",
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
  partner_missing:
    "der Name des Vertragspartners fehlt. Ergänzen Sie den Briefkopf. Oder erstellen Sie den Entwurf mit eingetragenem Vertragspartner neu",
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

/** How a contract was concluded, as the office reads it. */
const CONCLUSION_METHOD_LABELS: Record<ConclusionMethod, string> = {
  mit_anmeldung: "bei der Annahme der Anmeldung",
  einfach_elektronisch: "mit der Unterschrift der Eltern",
  papier: "mit dem unterschriebenen Vertrag auf Papier",
};

/**
 * The conclusion in one line: „Am 06.11.2026 um 16:12 Uhr, bei der Annahme
 * der Anmeldung“.
 */
export function formatConclusion(conclusion: ContractConclusion): string {
  return `Am ${formatBerlinDate(conclusion.at)} um ${formatBerlinTime(conclusion.at)} Uhr, ${CONCLUSION_METHOD_LABELS[conclusion.method]}`;
}

/** How the contracts of a school come about (#4680). */
export const CONCLUSION_MODE_LABELS: Record<ConclusionMode, string> = {
  bei_annahme: "Gilt bei der Annahme",
  nachtraeglich: "Sie verschicken ihn",
};

export const CONCLUSION_MODE_TONES: Record<ConclusionMode, StatusBadgeTone> = {
  bei_annahme: "green",
  nachtraeglich: "gray",
};
