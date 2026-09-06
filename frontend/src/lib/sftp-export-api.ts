// Übertragung der Zeitkonten-Exporte an die SFTP-Gegenstelle (#3050).
// Gleiche Berechtigung wie der Export selbst (time_tracking:manage); die
// übertragene Datei ist dieselbe, die der Download liefert.

import { sessionFetch } from "./session-cache";

type ExportFormat = "csv" | "xlsx" | "datev_lodas" | "datev_lug";

interface BackendSFTPStatus {
  enabled: boolean;
  ready: boolean;
  host?: string;
  port?: number;
  remote_directory?: string;
  missing_settings?: string[];
}

export interface SFTPStatus {
  /** Die Übertragung ist eingeschaltet. */
  enabled: boolean;
  /** Eingeschaltet UND vollständig eingerichtet — nur dann ist sie möglich. */
  ready: boolean;
  host?: string;
  remoteDirectory?: string;
  /** Schlüssel der noch leeren Einstellungen, in Formularreihenfolge. */
  missingSettings: string[];
}

interface BackendTransferOutcome {
  transferred: boolean;
  filename: string;
  byte_size: number;
  target_host?: string;
  target_directory?: string;
  reason?: string;
}

export interface TransferOutcome {
  transferred: boolean;
  filename: string;
  targetHost?: string;
  targetDirectory?: string;
  /** Stabiler Grund-Code, leer bei Erfolg. */
  reason?: string;
}

/**
 * Deutsche Sätze zu den Grund-Codes des Backends.
 *
 * Bewusst ohne technische Details: Was die Person tun kann, steht im Text;
 * warum es technisch scheiterte, steht im Log. Die Codes sind ein Vertrag mit
 * dem Backend und dem Übertragungsprotokoll.
 */
const FAILURE_MESSAGES: Record<string, string> = {
  not_configured:
    "Die Übertragung ist noch nicht eingerichtet. Ein Admin kann sie in den Einstellungen unter System, Bereich Schnittstellen, einschalten.",
  address_denied:
    "Diese Adresse ist nicht erlaubt. Möglich sind nur Adressen im Internet, nicht im eigenen Netz der Schule.",
  host_key_mismatch:
    "Die Gegenstelle konnte nicht sicher erkannt werden. Bitte prüfen Sie den Fingerabdruck in den Einstellungen unter System. Es wurde nichts übertragen.",
  authentication_rejected:
    "Die Anmeldung wurde abgelehnt. Bitte prüfen Sie Benutzername und Passwort in den Einstellungen unter System.",
  connection_failed:
    "Die Gegenstelle war nicht erreichbar. Bitte versuchen Sie es später noch einmal.",
  upload_failed:
    "Die Datei konnte nicht abgelegt werden. Bitte prüfen Sie den Zielordner in den Einstellungen unter System.",
  file_too_large: "Die Datei ist zu groß für die Übertragung.",
  internal_error:
    "Das hat leider nicht geklappt. Bitte versuchen Sie es noch einmal.",
};

/** Verständlicher Satz zu einem Grund-Code. */
export function transferFailureMessage(reason: string | undefined): string {
  if (!reason) return FAILURE_MESSAGES.internal_error!;
  return FAILURE_MESSAGES[reason] ?? FAILURE_MESSAGES.internal_error!;
}

export async function fetchSFTPStatus(): Promise<SFTPStatus> {
  const response = await sessionFetch(
    "/api/staff/time-tracking/export/sftp-status",
  );
  if (!response.ok) {
    throw new Error(`Failed to fetch SFTP status: ${response.statusText}`);
  }
  const json = (await response.json()) as { data: BackendSFTPStatus };
  return {
    enabled: json.data.enabled,
    ready: json.data.ready,
    host: json.data.host,
    remoteDirectory: json.data.remote_directory,
    missingSettings: json.data.missing_settings ?? [],
  };
}

interface TransferParams {
  year: number;
  month: number;
  format: ExportFormat;
  granularity?: "month" | "day";
  timeFormat?: "hhmm" | "decimal";
  /** Ganzes Jahr statt eines Monats. */
  wholeYear?: boolean;
}

export async function transferExportViaSFTP(
  params: TransferParams,
): Promise<TransferOutcome> {
  // Die Auswahl reist im Body, nicht in der Query: die POST-Proxy-Route
  // reicht den Body weiter, den Query-String aber nicht.
  const body: Record<string, string | number> = {
    year: params.year,
    format: params.format,
  };
  if (!params.wholeYear) {
    body.month = params.month;
  }
  if (params.granularity) {
    body.granularity = params.granularity;
  }
  if (params.timeFormat) {
    body.time_format = params.timeFormat;
  }

  const response = await sessionFetch("/api/staff/time-tracking/export/sftp", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  if (!response.ok) {
    throw new Error(`Transfer request failed: ${response.statusText}`);
  }
  const json = (await response.json()) as {
    data?: BackendTransferOutcome;
  };
  // Die Hülle muss ein `data` tragen. Fehlt es, hat die Route-Hülle das
  // Ergebnis für eine fertige Antwort gehalten — das passiert, sobald das
  // Payload ein Feld `success` hätte. Lieber laut scheitern als eine
  // Übertragung als geglückt zu melden, die niemand geprüft hat.
  if (!json.data) {
    throw new Error("Transfer response is missing its data envelope");
  }
  return {
    transferred: json.data.transferred,
    filename: json.data.filename,
    targetHost: json.data.target_host,
    targetDirectory: json.data.target_directory,
    reason: json.data.reason,
  };
}
