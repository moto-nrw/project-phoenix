/**
 * Die deutschen Beschriftungen der Anfragenliste an einer Stelle (#2267):
 * Artnamen, Überschriften der Abschnitte, Gründe des Backends. Getrennt von
 * den Komponenten, damit ein Text nicht in drei Dateien gleichzeitig gepflegt
 * werden muss und in Tests direkt geprüft werden kann.
 */

import { formatDate } from "~/lib/date-helpers";
import type {
  AggregatedRequestType,
  BulkIneligibleReason,
} from "~/lib/change-request-list-api";

export const REQUEST_TYPE_LABELS: Record<AggregatedRequestType, string> = {
  master_data: "Stammdaten",
  care_schedule: "Betreuungszeiten",
  offering: "Angebote",
  excused: "Abwesenheit",
  direct_correction: "Direkt-Korrektur",
  enrollment: "Anmeldung",
  care_withdrawal: "Abmeldung",
};

/**
 * Warum eine Anfrage nur einzeln entschieden werden kann. Der Code kommt vom
 * Backend; unbekannte Codes fallen auf dessen eigenen Text zurück, damit ein
 * neuer Grund sichtbar bleibt statt zu verschwinden.
 */
const BULK_INELIGIBLE_TEXTS: Record<string, string> = {
  past: "Diese Anfrage betrifft nur vergangene Tage.",
  stale: "Diese Anfrage wurde inzwischen geändert.",
  conflict: "Diese Anfrage widerspricht einer anderen Anfrage.",
  single_only: "Diese Art wird immer einzeln geprüft.",
  child_unavailable: "Für dieses Kind ist gerade keine Freigabe möglich.",
  access_revoked:
    "Die anfragende Bezugsperson hat keinen Zugriff mehr auf dieses Kind.",
};

export function bulkIneligibleText(
  code: BulkIneligibleReason | undefined,
  backendText: string | undefined,
): string {
  if (code && BULK_INELIGIBLE_TEXTS[code]) return BULK_INELIGIBLE_TEXTS[code];
  return backendText ?? "Diese Anfrage wird einzeln entschieden.";
}

/**
 * Der Wochentag im Widerspruchs-Schlüssel kommt als ISO-Nummer (1 = Montag).
 * Die Kurzformen bleiben stehen, damit ein älterer Aufbau nicht als Rohwert
 * durchschlägt.
 */
const WEEKDAY_LABELS: Record<string, string> = {
  "1": "Montag",
  "2": "Dienstag",
  "3": "Mittwoch",
  "4": "Donnerstag",
  "5": "Freitag",
  "6": "Samstag",
  "7": "Sonntag",
  mon: "Montag",
  tue: "Dienstag",
  wed: "Mittwoch",
  thu: "Donnerstag",
  fri: "Freitag",
  sat: "Samstag",
  sun: "Sonntag",
};

const MASTER_DATA_FIELD_LABELS: Record<string, string> = {
  first_name: "Vorname",
  last_name: "Nachname",
  birthday: "Geburtsdatum",
  school_class: "Klasse",
  health_info: "Gesundheitshinweise",
  email: "E-Mail",
  primary: "Telefonnummer",
  address_street: "Straße",
  address_city: "Ort",
  address_postal_code: "PLZ",
  preferred_contact_method: "Kontaktweg",
  language_preference: "Sprache",
  allowed_departure_modes: "Dauerhafte Gehzeiten",
};

const ISO_DATE = /^\d{4}-\d{2}-\d{2}$/;

/**
 * Überschrift einer Widerspruchsgruppe aus dem Schlüssel des Backends. Die
 * Schlüssel sind stabil (`absence:<Datum>`, `care:<Wochentag>:<Art>`,
 * `md:<Ziel>:<Feld>`, `offer:<Id>`, `pickup:<Datum>`); alles Unbekannte
 * bleibt als Rohwert stehen, statt eine falsche Überschrift zu erfinden.
 */
export function conflictGroupLabel(key: string): string {
  const [prefix, ...rest] = key.split(":");
  switch (prefix) {
    case "absence":
      return `Abwesenheit am ${formatDateSafe(rest[0])}`;
    case "pickup":
      return `Abholzeit am ${formatDateSafe(rest[0])}`;
    case "care": {
      const weekday = WEEKDAY_LABELS[rest[0] ?? ""] ?? rest[0] ?? "";
      return weekday ? `Betreuungszeit am ${weekday}` : "Betreuungszeit";
    }
    case "md": {
      const field = rest[1] ?? rest[0] ?? "";
      return (MASTER_DATA_FIELD_LABELS[field] ?? field) || "Stammdaten";
    }
    case "offer":
      return "Angebot";
    default:
      return key;
  }
}

function formatDateSafe(raw: string | undefined): string {
  return raw && ISO_DATE.test(raw) ? formatDate(raw) : (raw ?? "");
}

/**
 * Womit die OGS einen eigenen Wert einträgt — je Art ein passendes Feld: eine
 * Auswahl für den Anwesenheitsstand, eine Uhrzeit für Zeiten, ein Textfeld für
 * Stammdaten, und bei einem Angebot ein Gültigkeitsdatum plus die Wochentage.
 * Für das Angebot reicht das, weil der Schlüssel `offer:<id>` genau EIN
 * Angebot benennt: es braucht keinen Katalog und keinen zweiten Dialog.
 */
export type StaffValueInput = "status" | "time" | "text" | "offering";

export function conflictStaffValueInput(key: string): StaffValueInput {
  switch (key.split(":")[0]) {
    case "absence":
      return "status";
    case "care":
    case "pickup":
      return "time";
    case "offer":
      return "offering";
    default:
      return "text";
  }
}

/** Die Angebots-Kennung aus einem `offer:<id>`-Schlüssel. */
export function conflictOfferingID(key: string): string {
  return key.split(":")[1] ?? "";
}

export const OFFERING_NO_DAY_HINT = "Kein Tag = Abmeldung von diesem Angebot";

/** Die Stände, die eine Abwesenheit annehmen kann. */
export const ABSENCE_STATUS_OPTIONS = [
  { value: "present", label: "Da" },
  { value: "sick", label: "Krank" },
  { value: "excused", label: "Entschuldigt" },
  { value: "class_trip", label: "Ausflug" },
] as const;

export const DECISION_BLOCKED_BY_CONFLICT =
  "Diese Anfragen widersprechen sich. Legen Sie oben ein Ergebnis fest.";

export const CURRENT_VALUE_CHANGED_WARNING =
  "Die OGS hat diesen Wert nach der Anfrage geändert. Prüfen Sie, welcher Wert jetzt gelten soll.";

export const PAST_REQUEST_HINT =
  "Diese Anfrage betrifft nur vergangene Tage. Sie ändert nichts mehr.";

export const STALE_REQUEST_NOTICE =
  "Die Anfrage wurde inzwischen geändert. Die neue Fassung wird geladen.";
