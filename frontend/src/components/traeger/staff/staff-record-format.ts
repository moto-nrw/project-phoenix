/**
 * Wörter und Werte der Personalakten des Trägers (#4705): Feldnamen,
 * Gründe der Vorschläge und die Anzeige der Einträge im Verlauf.
 */
import type { StatusBadgeTone } from "~/components/ui/status-badge";
import { formatDate } from "~/lib/date-helpers";
import type {
  StaffChange,
  StaffMatchReason,
  StaffRecordField,
} from "~/lib/traeger-staff-api";

export const STAFF_FIELD_LABELS: Readonly<Record<StaffRecordField, string>> = {
  first_name: "Vorname",
  last_name: "Nachname",
  birthday: "Geburtstag",
  personnel_number: "Personalnummer",
  iban: "IBAN",
  tax_id: "Steuer-ID",
  social_security_number: "Sozialversicherungsnummer",
};

/** Warum moto Beschäftigungen zusammen vorschlägt, in einem Wort. */
export const STAFF_MATCH_LABELS: Readonly<Record<StaffMatchReason, string>> = {
  same_account: "Gleiches Konto",
  same_personnel_number: "Gleiche Personalnummer",
  same_name_birthday: "Gleicher Name und Geburtstag",
  single: "Eine Schule",
};

export const STAFF_MATCH_TONES: Readonly<
  Record<StaffMatchReason, StatusBadgeTone>
> = {
  same_account: "green",
  same_personnel_number: "blue",
  same_name_birthday: "orange",
  single: "gray",
};

/** Ein Feldwert für die Anzeige; ein Geburtstag als deutsches Datum. */
export function staffFieldValue(
  field: StaffRecordField,
  value: string,
): string {
  if (!value) return "–";
  return field === "birthday" ? formatDate(value) : value;
}

/** Ein Eintrag im Verlauf als ein Satz. */
export function staffChangeText(change: StaffChange): string {
  const school = change.schoolName || "einer Schule";
  if (change.kind === "linked") {
    return `Beschäftigung an der Schule ${school} verknüpft`;
  }
  const field =
    change.field === "school" ? "Schule" : STAFF_FIELD_LABELS[change.field];
  const value =
    change.field === "school"
      ? change.value
      : staffFieldValue(change.field, change.value);
  if (change.kind === "not_chosen") {
    return `${field} „${value}“ von ${school} nicht übernommen`;
  }
  return value === "–"
    ? `${field} eingetragen`
    : `${field} ersetzt, vorher „${value}“`;
}

/** Wie viele Schulen, als Wort. */
export function schoolCount(count: number): string {
  return count === 1 ? "1 Schule" : `${count} Schulen`;
}
