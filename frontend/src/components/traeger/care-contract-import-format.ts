// Texte und Zustände des Vertragsimports (#4723): die Felder eines Vertrags,
// die Zustände einer Zeile und was eine Zeile aufhält, in den Worten des
// Träger-Büros. Reine Anzeige; jede Prüfung macht das Backend.

import type { StatusBadgeTone } from "~/components/ui/status-badge";
import { formatDate } from "~/lib/date-helpers";
import type {
  ContractImportField,
  ContractImportResult,
  ContractImportRow,
  ContractImportRowStatus,
} from "~/lib/traeger-care-contract-import-api";

/** Die Felder in drei Gruppen, wie die Zuordnung sie zeigt. */
export const CONTRACT_IMPORT_GROUPS: readonly {
  readonly title: string;
  readonly fields: readonly ContractImportField[];
}[] = [
  {
    title: "Kind",
    fields: ["child_first_name", "child_last_name", "child_birth_date"],
  },
  {
    title: "Betreuung",
    fields: [
      "school",
      "module",
      "weekdays",
      "effective_from",
      "ends_on",
      "concluded_on",
    ],
  },
  {
    title: "Eltern und Abrechnung",
    fields: [
      "party_first_name",
      "party_last_name",
      "party_email",
      "debtor_number",
    ],
  },
];

/** Felder, die jede Datei als Spalte braucht. */
export const REQUIRED_CONTRACT_IMPORT_FIELDS: ReadonlySet<ContractImportField> =
  new Set([
    "child_first_name",
    "child_last_name",
    "effective_from",
    "party_last_name",
  ]);

export const CONTRACT_IMPORT_FIELD_LABELS: Record<ContractImportField, string> =
  {
    child_first_name: "Vorname des Kindes",
    child_last_name: "Nachname des Kindes",
    child_birth_date: "Geburtsdatum",
    school: "Schule",
    module: "Modul",
    weekdays: "Wochentage",
    effective_from: "Vertragsbeginn",
    ends_on: "Vertragsende",
    concluded_on: "Abgeschlossen am",
    party_first_name: "Vorname der Eltern",
    party_last_name: "Nachname der Eltern",
    party_email: "E-Mail der Eltern",
    debtor_number: "Debitorennummer",
  };

export const CONTRACT_IMPORT_FIELD_HINTS: Partial<
  Record<ContractImportField, string>
> = {
  child_birth_date: "Unterscheidet Kinder mit gleichem Namen.",
  weekdays: "Zum Beispiel „Mo-Do“. Ohne Spalte gelten die Tage des Moduls.",
  ends_on: "Ohne Datum endet der Vertrag mit dem Schuljahr des Moduls.",
  concluded_on: "Ohne Spalte gilt der Vertragsbeginn.",
  party_last_name: "Die Person, die den Vertrag unterschrieben hat.",
  party_email: "Nötig für Rechnungen und für die Debitorennummer.",
};

export const CONTRACT_IMPORT_STATUS: Record<
  ContractImportRowStatus,
  { readonly label: string; readonly tone: StatusBadgeTone }
> = {
  ready: { label: "Bereit", tone: "green" },
  duplicate: { label: "Schon vorhanden", tone: "gray" },
  conflict: { label: "Doppelt in der Datei", tone: "red" },
  invalid: { label: "Fehlerhaft", tone: "red" },
  imported: { label: "Übernommen", tone: "green" },
};

const WEEKDAY_SHORT: Record<string, string> = {
  mon: "Mo",
  tue: "Di",
  wed: "Mi",
  thu: "Do",
  fri: "Fr",
  sat: "Sa",
  sun: "So",
};

/** „Mo, Di, Do“; „–“ ohne Tage. */
export function contractImportWeekdays(weekdays: readonly string[]): string {
  if (weekdays.length === 0) return "–";
  return weekdays.map((day) => WEEKDAY_SHORT[day] ?? day).join(", ");
}

/** „ab 01.08.2026“ oder „01.08.2026 bis 31.07.2027“; „–“ ohne Beginn. */
export function contractImportPeriod(row: ContractImportRow): string {
  if (!row.effectiveFrom) return "–";
  return row.endsOn
    ? `${formatDate(row.effectiveFrom)} bis ${formatDate(row.endsOn)}`
    : `ab ${formatDate(row.effectiveFrom)}`;
}

/** Der Name des Kindes, „–“ ohne Namen. */
export function contractImportChild(row: ContractImportRow): string {
  const name = `${row.childFirstName} ${row.childLastName}`.trim();
  return name === "" ? "–" : name;
}

const MISSING_TEXT: Partial<Record<ContractImportField, string>> = {
  child_first_name: "Der Vorname des Kindes fehlt.",
  child_last_name: "Der Nachname des Kindes fehlt.",
  school: "Die Schule fehlt.",
  module: "Das Modul fehlt.",
  effective_from: "Der Vertragsbeginn fehlt.",
  party_last_name: "Der Nachname der Eltern fehlt.",
};

const INVALID_TEXT: Partial<Record<ContractImportField, string>> = {
  child_birth_date: "Das Geburtsdatum ist kein Datum. Beispiel: 03.05.2018.",
  effective_from: "Der Vertragsbeginn ist kein Datum. Beispiel: 01.08.2026.",
  ends_on: "Das Vertragsende ist kein Datum. Beispiel: 31.07.2027.",
  concluded_on: "„Abgeschlossen am“ ist kein Datum. Beispiel: 15.06.2026.",
  weekdays: "Die Wochentage sind nicht lesbar. Beispiel: Mo-Do oder Mo, Mi.",
  party_email: "Die E-Mail-Adresse ist nicht gültig.",
};

const UNKNOWN_TEXT: Partial<Record<ContractImportField, string>> = {
  school: "Diese Schule gehört nicht zu Ihrem Träger.",
  module: "Dieses Modul gibt es an der Schule nicht.",
};

const AMBIGUOUS_TEXT: Partial<Record<ContractImportField, string>> = {
  school: "Mehrere Schulen heißen so. Bitte eine Schule für die Datei wählen.",
  module: "Mehrere Module der Schule heißen so.",
};

/** Was eine Zeile aufhält, als ein Satz. */
export function contractImportProblemText(
  problem: ContractImportRow["problems"][number],
  row: ContractImportRow,
): string {
  const label = CONTRACT_IMPORT_FIELD_LABELS[problem.field] ?? problem.field;
  switch (problem.code) {
    case "missing":
      return MISSING_TEXT[problem.field] ?? `${label} fehlt.`;
    case "invalid":
      return INVALID_TEXT[problem.field] ?? `${label} prüfen.`;
    case "too_long":
      return `${label} ist zu lang.`;
    case "future":
      return `${label} liegt in der Zukunft.`;
    case "unknown":
      return UNKNOWN_TEXT[problem.field] ?? `${label} prüfen.`;
    case "ambiguous":
      return AMBIGUOUS_TEXT[problem.field] ?? `${label} ist nicht eindeutig.`;
    case "conditions_missing":
      return "Für das Modul fehlen die Vertragsbedingungen.";
    case "conditions_differ":
      return "Die Module haben verschiedene Vertragsbedingungen.";
    case "not_offered":
      return "Das Modul läuft nicht an allen genannten Wochentagen.";
    case "before_start":
      return "Das Vertragsende liegt vor dem Vertragsbeginn.";
    case "ended":
      return "Der Vertrag ist schon beendet. moto übernimmt nur laufende Verträge.";
    case "outside_phase":
      return "Das Schuljahr des Moduls ist schon vorbei.";
    case "repeated":
      return row.repeatsLine === null
        ? "Das Kind steht mehrmals in der Datei."
        : `Das Kind steht schon in Zeile ${row.repeatsLine}.`;
    default:
      return `${label} prüfen.`;
  }
}

/** Was der Import mit der Debitorennummer einer Zeile macht. */
export function contractImportDebtorText(
  row: ContractImportRow,
): string | null {
  switch (row.debtorNumberStatus) {
    case "new":
      return `Debitorennummer ${row.debtorNumber} wird eingetragen.`;
    case "same":
      return `Debitorennummer ${row.debtorNumber} ist schon eingetragen.`;
    case "differs":
      return "Debitorennummer: In moto steht schon eine andere Nummer. moto ändert sie nicht.";
    case "rows_differ":
      return "Debitorennummer: In der Datei stehen verschiedene Nummern für denselben Zahler. moto übernimmt keine.";
    case "invalid":
      return "Debitorennummer passt nicht zu Ihrem Nummernformat.";
    case "no_family":
      return "Debitorennummer nicht übernommen: Die E-Mail der Eltern fehlt.";
    default:
      return null;
  }
}

export type ContractImportFilter = "all" | "ready" | "problems" | "duplicate";

/** Ob eine Zeile zum gewählten Filter gehört. */
export function matchesContractImportFilter(
  row: ContractImportRow,
  filter: ContractImportFilter,
): boolean {
  switch (filter) {
    case "all":
      return true;
    case "ready":
      return row.status === "ready" || row.status === "imported";
    case "problems":
      return row.status === "conflict" || row.status === "invalid";
    default:
      return row.status === "duplicate";
  }
}

/** „1 Vertrag“, „12 Verträge“. */
export function contractCountLabel(count: number): string {
  return count === 1 ? "1 Vertrag" : `${count} Verträge`;
}

/** Zeilen, denen die Vertragsbedingungen ihres Moduls fehlen. */
export function rowsWithoutConditions(result: ContractImportResult): number {
  return result.rows.filter((row) =>
    row.problems.some((problem) => problem.code === "conditions_missing"),
  ).length;
}

/** Eine Beispieldatei mit den Spalten, die moto von selbst erkennt. */
export const CONTRACT_IMPORT_SAMPLE = [
  "Vorname Kind;Nachname Kind;Geburtsdatum;Schule;Modul;Wochentage;Vertragsbeginn;Vertragsende;Vorname Eltern;Nachname Eltern;E-Mail;Debitorennummer",
  "Mia;Beispiel;03.05.2018;Grundschule Nord;OGS bis 16 Uhr;Mo-Do;01.08.2026;;Eva;Beispiel;eva.beispiel@example.org;70001",
  "Ben;Muster;11.11.2019;Grundschule Nord;OGS bis 14 Uhr;;01.08.2026;31.01.2027;Jan;Muster;jan.muster@example.org;70002",
].join("\r\n");
