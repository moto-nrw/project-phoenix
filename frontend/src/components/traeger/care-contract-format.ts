/**
 * Display helpers of the care contracts in the Träger portal (#4678). Term,
 * notice, fee and module texts come from the backend, which prints the same
 * texts into the contract PDF.
 */

import type { StatusBadgeTone } from "~/components/ui/status-badge";
import { contractWeekdays } from "~/lib/care-contract-weekdays";
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
  SendSkipReason,
  TemplateIssue,
  Weekday,
} from "~/lib/traeger-care-contracts-api";

export const CONTRACT_STATUS_LABELS: Record<CareContractStatus, string> = {
  entwurf: "Entwurf",
  versendet: "Versendet",
  aktiv: "Aktiv",
  gekuendigt: "Gekündigt",
  widerrufen: "Widerrufen",
  beendet: "Beendet",
};

export const CONTRACT_STATUS_TONES: Record<
  CareContractStatus,
  StatusBadgeTone
> = {
  entwurf: "gray",
  versendet: "blue",
  aktiv: "green",
  gekuendigt: "orange",
  widerrufen: "red",
  beendet: "outline",
};

/** The order of the status filter: what the office works on first. */
export const CONTRACT_STATUS_ORDER: readonly CareContractStatus[] = [
  "entwurf",
  "versendet",
  "aktiv",
  "gekuendigt",
  "widerrufen",
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

const WEEKDAY_LONG: Record<Weekday, string> = {
  mon: "Montag",
  tue: "Dienstag",
  wed: "Mittwoch",
  thu: "Donnerstag",
  fri: "Freitag",
  sat: "Samstag",
  sun: "Sonntag",
};

export interface ServiceDay {
  readonly weekday: Weekday;
  readonly label: string;
  readonly services: readonly ContractService[];
}

/** The weekdays of a contract (`contractWeekdays`) with their modules. */
export function serviceDays(
  services: readonly ContractService[],
  include: readonly Weekday[] = [],
): ServiceDay[] {
  return contractWeekdays(services, include).map((weekday) => ({
    weekday,
    label: WEEKDAY_LONG[weekday],
    services: services.filter((service) => service.weekday === weekday),
  }));
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

export const TEMPLATE_ISSUE_LABELS: Record<TemplateIssue, string> = {
  template_missing: "Vertragsbedingungen fehlen",
  templates_differ: "Bedingungen der Module weichen ab",
  conclusion_automatic: "Bei der Annahme",
  acceptance_missing: "Zustimmung zum Vertrag fehlt",
  acceptance_changed: "Bedingungen nach der Anmeldung geändert",
  recipient_missing: "E-Mail-Adresse der Eltern fehlt",
  mail_blocked: "Mail-Adresse gesperrt",
  mail_stopped: "Mails gestoppt",
};

/**
 * The reason in one plain sentence, with the next step, where the label
 * alone does not tell what happens (#4680).
 */
export const TEMPLATE_ISSUE_REASONS: Partial<Record<TemplateIssue, string>> = {
  conclusion_automatic: "Der Vertrag entsteht in wenigen Minuten von selbst.",
  recipient_missing:
    "Die Schule ergänzt die E-Mail-Adresse in der Anmeldung. Danach versucht moto es erneut. Bis dahin entsteht kein Vertrag.",
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

export const SEND_SKIP_LABELS: Record<SendSkipReason, string> = {
  recipient_missing: "keine E-Mail-Adresse",
  partner_missing:
    "der Name des Vertragspartners fehlt. Ergänzen Sie den Briefkopf. Oder erstellen Sie den Entwurf mit eingetragenem Vertragspartner neu",
  mail_pending: "die letzte Mail wird noch gesendet",
  mail_delayed: "die letzte Mail ist verzögert und kommt später an",
  mail_blocked: "eine Mail an diese Adresse würde nicht rausgehen",
  mail_block_unreadable: "der Stand der Adresse lässt sich gerade nicht prüfen",
  not_sendable: "lässt sich nicht mehr senden",
};

export function childName(contract: {
  readonly childFirstName: string;
  readonly childLastName: string;
}): string {
  return `${contract.childFirstName} ${contract.childLastName}`.trim();
}

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

export const CONCLUSION_MODE_LABELS: Record<ConclusionMode, string> = {
  bei_annahme: "Bei der Annahme",
  nachtraeglich: "Später verschicken",
};

export const CONCLUSION_MODE_TONES: Record<ConclusionMode, StatusBadgeTone> = {
  bei_annahme: "green",
  nachtraeglich: "gray",
};

export const TERMINATION_KIND_LABELS = {
  ordentlich: "Ordentlich",
  sonderkuendigung: "Sonderkündigung",
  ausserordentlich: "Außerordentlich, aus wichtigem Grund",
} as const;
