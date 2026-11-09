/**
 * Client of the finance exports (#4693): the field mapping of the
 * Sollstellung, the file numbers of the Träger, the released runs with
 * their last export, the export protocol, the download of a Sollstellung and
 * the history of one invoice. Backend int64 IDs leave here as strings;
 * calendar dates stay "YYYY-MM-DD", instants ISO strings.
 */

import { traegerRequest } from "./traeger-api";
import { type BillingFile, billingFile } from "./traeger-billing-api";

/** The kind of run, as the path names it. */
export type ExportRunPathKind = "care-runs" | "holiday-runs" | "notice-months";
type ExportRunKind = "care" | "holiday" | "notice";
export type ExportFormat = "sollstellung" | "pain008";

export type SollField =
  | "invoice_number"
  | "invoice_date"
  | "due_on"
  | "amount"
  | "debtor_number"
  | "recipient_name"
  | "recipient_street"
  | "recipient_postal_code"
  | "recipient_city"
  | "child_name"
  | "school_name"
  | "service_from"
  | "service_until"
  | "payment_method"
  | "mandate_reference"
  | "document_kind"
  | "file_number"
  | "constant";

export type SollDelimiter = ";" | "," | "\t" | "|";
export type SollDecimal = "," | ".";
export type SollDateFormat = "dd.mm.yyyy" | "yyyy-mm-dd";
export type SollEncoding = "utf-8-bom" | "utf-8" | "windows-1252";

export interface SollColumn {
  readonly field: SollField;
  readonly header: string;
  /** Only a constant column carries a value. */
  readonly value: string;
}

export interface SollMapping {
  readonly columns: readonly SollColumn[];
  readonly delimiter: SollDelimiter;
  readonly decimalSeparator: SollDecimal;
  readonly dateFormat: SollDateFormat;
  readonly encoding: SollEncoding;
  readonly headerRow: boolean;
  readonly fileNamePattern: string;
}

export interface ExportSettings {
  readonly mapping: SollMapping;
  /** Whether the Träger set a mapping of its own. */
  readonly custom: boolean;
  readonly updatedAt: string | null;
  /** The number the next file gets. */
  readonly nextFileNumber: number;
  /** A start value was set or a file was made: the start is fixed. */
  readonly fileNumbersStarted: boolean;
}

export interface ExportProtocolEntry {
  readonly id: string;
  readonly fileNumber: number;
  readonly format: ExportFormat;
  readonly runKind: ExportRunKind;
  readonly runId: string;
  readonly runLabel: string;
  readonly fileName: string;
  readonly invoiceCount: number;
  readonly totalCents: number;
  /** Message ID of a PAIN.008 file. */
  readonly reference: string;
  /** Number of the earliest file of the run with the same content, else null. */
  readonly repeatOf: number | null;
  readonly exportedByYou: boolean;
  readonly exportedByAccount: boolean;
  readonly exportedAt: string;
  readonly submittedAt: string | null;
}

export interface ExportRun {
  readonly runKind: ExportRunKind;
  readonly pathKind: ExportRunPathKind;
  readonly runId: string;
  readonly label: string;
  /** "YYYY-MM-DD". */
  readonly issuedOn: string;
  readonly invoiceCount: number;
  readonly totalCents: number;
  readonly lastSollstellung: ExportProtocolEntry | null;
  readonly exports: number;
}

export type InvoiceHistoryKind = "care-invoices" | "holiday-documents";
type InvoiceStepKind =
  "issued" | "mail_requested" | "sent" | "exported" | "submitted" | "cancelled";

export interface InvoiceStep {
  readonly kind: InvoiceStepKind;
  readonly at: string;
  readonly byYou: boolean;
  readonly byAccount: boolean;
  readonly positions: number;
  readonly note: string;
  readonly file: ExportProtocolEntry | null;
}

export interface InvoiceHistory {
  readonly number: string;
  readonly steps: readonly InvoiceStep[];
}

interface BackendColumn {
  field: SollField;
  header: string;
  value?: string;
}

interface BackendMapping {
  columns: BackendColumn[] | null;
  delimiter: SollDelimiter;
  decimal_separator: SollDecimal;
  date_format: SollDateFormat;
  encoding: SollEncoding;
  header_row: boolean;
  file_name_pattern: string;
}

interface BackendSettings {
  sollstellung: {
    mapping: BackendMapping;
    custom: boolean;
    updated_at: string | null;
  };
  file_numbers: { next: number; started: boolean };
}

interface BackendEntry {
  id: number;
  file_number: number;
  format: ExportFormat;
  run_kind: ExportRunKind;
  run_id: number;
  run_label: string;
  file_name: string;
  invoice_count: number;
  total_cents: number;
  reference?: string;
  repeat_of?: number;
  exported_by_account_id: number | null;
  exported_by_you: boolean;
  exported_at: string;
  submitted_at: string | null;
}

interface BackendRun {
  run_kind: ExportRunKind;
  run_id: number;
  label: string;
  issued_on: string;
  invoice_count: number;
  total_cents: number;
  last_sollstellung: BackendEntry | null;
  exports: number;
}

interface BackendHistory {
  number: string;
  entries:
    | {
        kind: InvoiceStepKind;
        at: string;
        account_id: number | null;
        by_you: boolean;
        positions?: number;
        note?: string;
        file?: BackendEntry;
      }[]
    | null;
}

const BASE = "/api/traeger/finance-exports";

const PATH_KINDS: Record<ExportRunKind, ExportRunPathKind> = {
  care: "care-runs",
  holiday: "holiday-runs",
  notice: "notice-months",
};

function mappingFromBackend(mapping: BackendMapping): SollMapping {
  return {
    columns: (mapping.columns ?? []).map((column) => ({
      field: column.field,
      header: column.header,
      value: column.value ?? "",
    })),
    delimiter: mapping.delimiter,
    decimalSeparator: mapping.decimal_separator,
    dateFormat: mapping.date_format,
    encoding: mapping.encoding,
    headerRow: mapping.header_row,
    fileNamePattern: mapping.file_name_pattern,
  };
}

function mappingToBackend(mapping: SollMapping): BackendMapping {
  return {
    columns: mapping.columns.map((column) => ({
      field: column.field,
      header: column.header,
      ...(column.field === "constant" ? { value: column.value } : {}),
    })),
    delimiter: mapping.delimiter,
    decimal_separator: mapping.decimalSeparator,
    date_format: mapping.dateFormat,
    encoding: mapping.encoding,
    header_row: mapping.headerRow,
    file_name_pattern: mapping.fileNamePattern,
  };
}

function settingsFromBackend(settings: BackendSettings): ExportSettings {
  return {
    mapping: mappingFromBackend(settings.sollstellung.mapping),
    custom: settings.sollstellung.custom,
    updatedAt: settings.sollstellung.updated_at,
    nextFileNumber: settings.file_numbers.next,
    fileNumbersStarted: settings.file_numbers.started,
  };
}

function entryFromBackend(entry: BackendEntry): ExportProtocolEntry {
  return {
    id: String(entry.id),
    fileNumber: entry.file_number,
    format: entry.format,
    runKind: entry.run_kind,
    runId: String(entry.run_id),
    runLabel: entry.run_label,
    fileName: entry.file_name,
    invoiceCount: entry.invoice_count,
    totalCents: entry.total_cents,
    reference: entry.reference ?? "",
    repeatOf: entry.repeat_of ?? null,
    exportedByYou: entry.exported_by_you,
    exportedByAccount: entry.exported_by_account_id !== null,
    exportedAt: entry.exported_at,
    submittedAt: entry.submitted_at,
  };
}

/** The mapping of the Sollstellung and the file numbers of the Träger. */
export async function fetchExportSettings(): Promise<ExportSettings> {
  return settingsFromBackend(
    await traegerRequest<BackendSettings>(`${BASE}/settings`),
  );
}

/** Stores the mapping; the backend names each wrong field. */
export async function saveSollMapping(
  mapping: SollMapping,
): Promise<ExportSettings> {
  return settingsFromBackend(
    await traegerRequest<BackendSettings>(`${BASE}/settings/sollstellung`, {
      method: "PUT",
      body: mappingToBackend(mapping),
    }),
  );
}

/** Returns the Träger to the default mapping. */
export async function resetSollMapping(): Promise<ExportSettings> {
  await traegerRequest(`${BASE}/settings/sollstellung`, { method: "DELETE" });
  return fetchExportSettings();
}

/** Sets the number of the first file, once. */
export async function setFileNumberStart(
  start: number,
): Promise<ExportSettings> {
  return settingsFromBackend(
    await traegerRequest<BackendSettings>(
      `${BASE}/settings/file-number-start`,
      { method: "PUT", body: { start } },
    ),
  );
}

/** The released runs, newest first, with their last Sollstellung. */
export async function fetchExportRuns(): Promise<ExportRun[]> {
  const body = await traegerRequest<{ runs: BackendRun[] | null }>(
    `${BASE}/runs`,
  );
  return (body.runs ?? []).map((run) => ({
    runKind: run.run_kind,
    pathKind: PATH_KINDS[run.run_kind],
    runId: String(run.run_id),
    label: run.label,
    issuedOn: run.issued_on,
    invoiceCount: run.invoice_count,
    totalCents: run.total_cents,
    lastSollstellung: run.last_sollstellung
      ? entryFromBackend(run.last_sollstellung)
      : null,
    exports: run.exports,
  }));
}

/** The export protocol, newest first. */
export async function fetchExportProtocol(): Promise<ExportProtocolEntry[]> {
  const body = await traegerRequest<{ files: BackendEntry[] | null }>(
    `${BASE}/files`,
  );
  return (body.files ?? []).map(entryFromBackend);
}

/** Downloads the Sollstellung of a run with the next file number. */
export async function downloadSollstellung(
  kind: ExportRunPathKind,
  runId: string,
): Promise<BillingFile> {
  return billingFile(
    `${BASE}/runs/${kind}/${encodeURIComponent(runId)}/sollstellung`,
    {},
    "sollstellung.csv",
  );
}

/** The history of an invoice, oldest step first. */
export async function fetchInvoiceHistory(
  kind: InvoiceHistoryKind,
  invoiceId: string,
): Promise<InvoiceHistory> {
  const body = await traegerRequest<BackendHistory>(
    `${BASE}/invoice-history/${kind}/${encodeURIComponent(invoiceId)}`,
  );
  return {
    number: body.number,
    steps: (body.entries ?? []).map((entry) => ({
      kind: entry.kind,
      at: entry.at,
      byYou: entry.by_you,
      byAccount: entry.account_id !== null,
      positions: entry.positions ?? 0,
      note: entry.note ?? "",
      file: entry.file ? entryFromBackend(entry.file) : null,
    })),
  };
}
