/**
 * Texts of the handover of finance exports to the recipient of the Träger
 * (#6824, ADR 0217): what each state and each reason code means for the
 * person in the office. The codes are a contract with the backend; the
 * transport's own error never reaches the screen.
 */

import type { StatusBadgeTone } from "~/components/ui/status-badge";
import type {
  HandoverState,
  RecipientField,
} from "~/lib/traeger-export-handovers-api";
import type { ExportFormat } from "~/lib/traeger-finance-exports-api";

/** The page of the recipient, below the exports. */
export const EXPORT_RECIPIENT_PATH = "/traeger/abrechnung/exporte/empfaenger";

export const HANDOVER_STATE_LABELS: Record<HandoverState, string> = {
  transferred: "Angekommen",
  failed: "Nicht angekommen",
  in_progress: "Wird übergeben",
  unknown: "Unklar",
};

export const HANDOVER_STATE_TONES: Record<HandoverState, StatusBadgeTone> = {
  transferred: "green",
  failed: "red",
  in_progress: "blue",
  unknown: "warning",
};

export const HANDOVER_FORMAT_LABELS: Record<ExportFormat, string> = {
  sollstellung: "Sollstellung",
  datev: "DATEV-Datei",
  pain008: "Lastschriftdatei",
};

/** What a failed handover means and what to do; the file did not arrive. */
const REASONS: Record<string, string> = {
  address_denied:
    "Diese Adresse ist nicht erlaubt. Möglich sind nur Adressen im Internet, nicht im eigenen Netz.",
  host_key_mismatch:
    "Der Empfänger konnte nicht sicher erkannt werden. Bitte prüfen Sie den Fingerabdruck beim Empfänger.",
  authentication_rejected:
    "Die Anmeldung wurde abgelehnt. Bitte prüfen Sie Benutzername und Passwort beim Empfänger.",
  connection_failed:
    "Der Empfänger war nicht erreichbar. Bitte versuchen Sie es später noch einmal.",
  upload_failed:
    "Die Datei konnte nicht abgelegt werden. Bitte prüfen Sie den Zielordner beim Empfänger.",
  file_too_large: "Die Datei ist zu groß für die Übergabe.",
  internal_error:
    "Das hat leider nicht geklappt. Bitte versuchen Sie es noch einmal.",
};

/** A sentence for a reason code; an unknown code reads as a failure. */
export function handoverReasonText(reason: string): string {
  return REASONS[reason] ?? REASONS.internal_error!;
}

/** The sentence of an unknown outcome: nothing retries it on its own. */
export const HANDOVER_UNKNOWN_TEXT =
  "Ob die Datei angekommen ist, ist unklar. Fragen Sie beim Empfänger nach, bevor Sie sie noch einmal übergeben.";

const RECIPIENT_FIELD_LABELS: Record<RecipientField, string> = {
  host: "Adresse",
  port: "Port",
  username: "Benutzername",
  password: "Passwort",
  remote_directory: "Zielordner",
  host_key_fingerprint: "Fingerabdruck",
};

/** „Adresse, Passwort und Zielordner“ for the missing fields. */
export function missingFieldsText(fields: readonly RecipientField[]): string {
  const labels = fields.map((field) => RECIPIENT_FIELD_LABELS[field]);
  if (labels.length <= 1) return labels.join("");
  return `${labels.slice(0, -1).join(", ")} und ${labels.at(-1)}`;
}

/** Where a file goes, as one line: „dateien.beispiel.de, Ordner /eingang“. */
export function destinationText(host: string, directory: string): string {
  return directory ? `${host}, Ordner ${directory}` : host;
}
