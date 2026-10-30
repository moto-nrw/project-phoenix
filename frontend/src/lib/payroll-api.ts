// Payroll configuration status (#1417 Tranche 2b): the /payroll page's data
// source. Requires config:manage — callers gate rendering on the permission
// so no request fires without it. Values are written to the generic settings
// route (savePayrollSetting) using the setting keys this status carries.

import { apiErrorFromResponse } from "./api-error";
import { sessionFetch } from "./session-cache";
import { transferFailureMessage } from "./sftp-export-api";

interface BackendPayrollCategoryStatus {
  id: string;
  label: string;
  number: string;
  unit?: string;
  unit_required: boolean;
  setting_key: string;
  unit_setting_key?: string;
}

interface BackendPayrollStatus {
  categories: BackendPayrollCategoryStatus[];
  beraternummer: string;
  mandantennummer: string;
  lodas_header_complete: boolean;
  configured_categories: number;
  total_categories: number;
  staff_total: number;
  staff_without_personnel_number: number;
  dispatch?: BackendPayrollDispatchSettings;
}

interface BackendPayrollDispatchSettings {
  auto: boolean;
  channel: string;
  format: string;
  email: string;
}

interface PayrollCategoryStatus {
  id: string;
  label: string;
  number: string;
  unit: string;
  unitRequired: boolean;
  settingKey: string;
  unitSettingKey: string | null;
}

export interface PayrollStatus {
  categories: PayrollCategoryStatus[];
  beraternummer: string;
  mandantennummer: string;
  lodasHeaderComplete: boolean;
  configuredCategories: number;
  totalCategories: number;
  staffTotal: number;
  staffWithoutPersonnelNumber: number;
  dispatch: PayrollDispatchSettings;
}

/** Wege zum Lohnbüro (#4805); die Werte von `payroll.dispatch_channel`. */
export type PayrollDispatchChannel = "sftp" | "email";

/** Dateiformate; die Werte von `payroll.dispatch_format`. */
export type PayrollDispatchFormat = "csv" | "xlsx" | "datev_lodas" | "datev_lug";

/** Der Versand an das Lohnbüro nach dem Monatsabschluss (#4805). */
export interface PayrollDispatchSettings {
  auto: boolean;
  channel: PayrollDispatchChannel;
  format: PayrollDispatchFormat;
  email: string;
}

/** Setting-Schlüssel des Versands; das Backend prüft Typ und Muster. */
export const PAYROLL_DISPATCH_KEYS = {
  auto: "payroll.dispatch_auto",
  channel: "payroll.dispatch_channel",
  format: "payroll.dispatch_format",
  email: "payroll.dispatch_email",
} as const;

/** Dasselbe Muster wie `payroll.dispatch_email` im Backend; leer ist erlaubt. */
export const PAYROLL_DISPATCH_EMAIL_PATTERN =
  /^[^@\s]{1,64}@[^@\s]+\.[^@\s]{2,}$/;

export const PAYROLL_DISPATCH_CHANNEL_LABELS: Record<
  PayrollDispatchChannel,
  string
> = {
  sftp: "SFTP",
  email: "E-Mail",
};

/** Dieselben Namen wie die Auswahl im Backend-Schema. */
export const PAYROLL_DISPATCH_FORMAT_LABELS: Record<
  PayrollDispatchFormat,
  string
> = {
  csv: "CSV",
  xlsx: "Excel",
  datev_lodas: "DATEV LODAS",
  datev_lug: "DATEV Lohn und Gehalt",
};

function dispatchChannel(value: string | undefined): PayrollDispatchChannel {
  return value === "email" ? "email" : "sftp";
}

function dispatchFormat(value: string | undefined): PayrollDispatchFormat {
  return value !== undefined && value in PAYROLL_DISPATCH_FORMAT_LABELS
    ? (value as PayrollDispatchFormat)
    : "csv";
}

export function mapPayrollStatus(data: BackendPayrollStatus): PayrollStatus {
  return {
    categories: (data.categories ?? []).map((cat) => ({
      id: cat.id,
      label: cat.label,
      number: cat.number,
      unit: cat.unit ?? "",
      unitRequired: cat.unit_required,
      settingKey: cat.setting_key,
      unitSettingKey: cat.unit_setting_key ?? null,
    })),
    beraternummer: data.beraternummer,
    mandantennummer: data.mandantennummer,
    lodasHeaderComplete: data.lodas_header_complete,
    configuredCategories: data.configured_categories,
    totalCategories: data.total_categories,
    staffTotal: data.staff_total,
    staffWithoutPersonnelNumber: data.staff_without_personnel_number,
    // Die Vorgaben des Backends (aus, SFTP, CSV), falls ein älteres Backend
    // den Block noch nicht liefert.
    dispatch: {
      auto: data.dispatch?.auto ?? false,
      channel: dispatchChannel(data.dispatch?.channel),
      format: dispatchFormat(data.dispatch?.format),
      email: data.dispatch?.email ?? "",
    },
  };
}

export async function fetchPayrollStatus(): Promise<PayrollStatus> {
  const response = await sessionFetch("/api/settings/payroll-status");
  if (!response.ok) {
    throw await apiErrorFromResponse(
      response,
      `Failed to fetch payroll status: ${response.statusText}`,
    );
  }
  const json = (await response.json()) as { data: BackendPayrollStatus };
  return mapPayrollStatus(json.data);
}

/** Ein Einstellungswert der Abrechnung im JSON-Typ seines Schemas. */
export type PayrollSettingValue = string | boolean;

/**
 * Saves one payroll setting. Throws an ApiError with code, field errors and
 * request ID, so the page shows the catalog text (#2517). The value travels
 * in its schema type: the Lohnbüro switch as a boolean, everything else as a
 * string.
 */
export async function savePayrollSetting(
  key: string,
  value: PayrollSettingValue,
): Promise<void> {
  const response = await sessionFetch(
    `/api/settings/values/${encodeURIComponent(key)}`,
    {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ value }),
    },
  );
  if (!response.ok) {
    throw await apiErrorFromResponse(response, `Saving ${key} failed`);
  }
}

/**
 * Lohnartnummern that appear on more than one category. Not an error — a
 * Träger may legitimately book two categories on one Lohnart — but worth a
 * warning, because a duplicate usually is a typo that would double-book.
 */
export function duplicateLohnartNumbers(status: PayrollStatus): string[] {
  const seen = new Map<string, number>();
  for (const cat of status.categories) {
    if (cat.number === "") continue;
    seen.set(cat.number, (seen.get(cat.number) ?? 0) + 1);
  }
  return [...seen.entries()]
    .filter(([, count]) => count > 1)
    .map(([number]) => number);
}

// ─── Letzter Versand an das Lohnbüro (#4805) ────────────────────────────────

export type PayrollDispatchState =
  "pending" | "mailing" | "sent" | "blocked" | "failed";

interface BackendPayrollDispatchView {
  year: number;
  month: number;
  correction: boolean;
  channel: string;
  format: string;
  recipient?: string;
  state: PayrollDispatchState;
  reason?: string;
  attempts: number;
  filename?: string;
  created_at: string;
  finished_at?: string;
  download_expires_at?: string;
  downloaded_at?: string;
}

export interface PayrollDispatchView {
  year: number;
  month: number;
  correction: boolean;
  channel: PayrollDispatchChannel;
  format: PayrollDispatchFormat;
  recipient: string | null;
  state: PayrollDispatchState;
  reason: string | null;
  attempts: number;
  filename: string | null;
  /** Zeitpunkte (RFC 3339), keine Kalendertage. */
  createdAt: string;
  finishedAt: string | null;
  downloadExpiresAt: string | null;
  downloadedAt: string | null;
}

export function mapPayrollDispatch(
  data: BackendPayrollDispatchView,
): PayrollDispatchView {
  return {
    year: data.year,
    month: data.month,
    correction: data.correction,
    channel: dispatchChannel(data.channel),
    format: dispatchFormat(data.format),
    recipient: data.recipient ?? null,
    state: data.state,
    reason: data.reason ?? null,
    attempts: data.attempts,
    filename: data.filename ?? null,
    createdAt: data.created_at,
    finishedAt: data.finished_at ?? null,
    downloadExpiresAt: data.download_expires_at ?? null,
    downloadedAt: data.downloaded_at ?? null,
  };
}

/**
 * Der neueste automatische Versand, `null` vor dem ersten. Braucht
 * time_tracking:manage oder config:manage.
 */
export async function fetchPayrollDispatch(): Promise<PayrollDispatchView | null> {
  const response = await sessionFetch(
    "/api/staff/time-tracking/payroll-dispatch",
  );
  if (!response.ok) {
    throw await apiErrorFromResponse(
      response,
      `Failed to fetch payroll dispatch: ${response.statusText}`,
    );
  }
  const json = (await response.json()) as {
    data: { latest: BackendPayrollDispatchView | null };
  };
  return json.data.latest ? mapPayrollDispatch(json.data.latest) : null;
}

/**
 * Sätze zu den Grund-Codes des Versands. Derselbe Wortlaut steht in der
 * Hinweis-E-Mail des Backends (payroll_dispatch_mail.go).
 */
const DISPATCH_REASON_TEXTS: Record<string, string> = {
  datev_incomplete:
    "Die DATEV-Angaben sind nicht vollständig. Zum Beispiel fehlt eine Personalnummer oder die Nummer einer Lohnart.",
  sftp_not_configured:
    "Die Übertragung per SFTP ist ausgeschaltet oder nicht vollständig eingetragen.",
  email_missing: "Für den Versand per E-Mail fehlt die Adresse des Lohnbüros.",
  mail_failed: "Die E-Mail an das Lohnbüro konnte nicht zugestellt werden.",
  internal_error: "Die Datei konnte nicht erstellt werden.",
};

/** Nach allen Versuchen per SFTP: erster Versuch und drei Wiederholungen. */
const SFTP_GAVE_UP_TEXT =
  "Die Übertragung an das Lohnbüro hat viermal nicht geklappt.";

/**
 * Warum ein Versand nicht ankam, als Satz; `null`, solange nichts schiefging.
 * Ein gescheiterter SFTP-Versand trägt den Grund der Übertragung
 * (connection_failed, host_key_mismatch, ...), dessen Text die
 * SFTP-Übertragung schon kennt.
 */
export function payrollDispatchReasonText(
  dispatch: Pick<PayrollDispatchView, "state" | "reason" | "channel">,
): string | null {
  if (dispatch.state !== "blocked" && dispatch.state !== "failed") return null;
  const reason = dispatch.reason ?? "internal_error";
  if (dispatch.state === "failed" && dispatch.channel === "sftp") {
    // Ohne genauen Grund bleibt es beim ersten Satz: „Bitte versuchen Sie es
    // noch einmal“ passt nicht zu einem Versand, den moto selbst wiederholt.
    return reason === "internal_error"
      ? SFTP_GAVE_UP_TEXT
      : `${SFTP_GAVE_UP_TEXT} ${transferFailureMessage(reason)}`;
  }
  return DISPATCH_REASON_TEXTS[reason] ?? DISPATCH_REASON_TEXTS.internal_error!;
}

const MONTH_FORMATTER = new Intl.DateTimeFormat("de-DE", {
  month: "long",
  year: "numeric",
  timeZone: "UTC",
});

/** „September 2026“ für Jahr und Monat (1 bis 12). */
export function payrollMonthLabel(year: number, month: number): string {
  return MONTH_FORMATTER.format(new Date(Date.UTC(year, month - 1, 1)));
}
