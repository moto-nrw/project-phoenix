/**
 * Client of the finance exports (#4693): the field mapping of the
 * Sollstellung, the file numbers of the Träger, the released runs with
 * their last export, the export protocol, the download of a Sollstellung or
 * a DATEV booking batch (#4694) and the history of one invoice. Backend int64 IDs leave here as strings;
 * calendar dates stay "YYYY-MM-DD", instants ISO strings.
 */

import { traegerRequest } from "./traeger-api";
import { type BillingFile, billingFile } from "./traeger-billing-api";

/** The kind of run, as the path names it. */
export type ExportRunPathKind = "care-runs" | "holiday-runs" | "notice-months";
type ExportRunKind = "care" | "holiday" | "notice";
export type ExportFormat = "sollstellung" | "pain008" | "datev";

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

/** Whether the DATEV values of the Träger make a valid file (#4694). */
export interface DatevReadiness {
  readonly ready: boolean;
  /** Keys of the settings that are empty. */
  readonly missingSettings: readonly string[];
  /** Keys of the settings whose value DATEV would refuse. */
  readonly invalidSettings: readonly string[];
}

/** An invoice DATEV would refuse, named by a refused download. */
export interface DatevBookingProblem {
  readonly documentNumber: string;
  readonly field:
    "debtor_number" | "document_date" | "document_number" | "amount";
  readonly reason: "missing" | "invalid" | "out_of_period";
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
  readonly datev: DatevReadiness;
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
  /** A DATEV file of "all bookings again" (#6433). */
  readonly datevAll: boolean;
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
  readonly lastDatev: ExportProtocolEntry | null;
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
  datev?: {
    ready: boolean;
    missing_settings: string[] | null;
    invalid_settings: string[] | null;
  };
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
  datev_all?: boolean;
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
  last_datev?: BackendEntry | null;
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
    datev: {
      ready: settings.datev?.ready ?? false,
      missingSettings: settings.datev?.missing_settings ?? [],
      invalidSettings: settings.datev?.invalid_settings ?? [],
    },
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
    datevAll: entry.datev_all ?? false,
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

/** One page of a list of the export page, newest first. */
export interface FinanceExportPage<T> {
  readonly items: readonly T[];
  /** Entries of the whole list, not only of this page. */
  readonly total: number;
  /** Offset of the next page, null on the last. */
  readonly nextOffset: number | null;
}

/** A page of runs also counts the runs without a Sollstellung. */
export interface FinanceExportRunPage extends FinanceExportPage<ExportRun> {
  readonly notExported: number;
}

interface BackendPage {
  total: number;
  next_offset: number | null;
}

function pagePath(path: string, offset: number): string {
  return offset > 0 ? `${path}?offset=${offset}` : path;
}

/** A page of the released runs, newest first, with their last Sollstellung. */
export async function fetchExportRuns(
  offset = 0,
): Promise<FinanceExportRunPage> {
  const body = await traegerRequest<
    BackendPage & { runs: BackendRun[] | null; not_exported: number }
  >(pagePath(`${BASE}/runs`, offset));
  return {
    items: (body.runs ?? []).map((run) => ({
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
      lastDatev: run.last_datev ? entryFromBackend(run.last_datev) : null,
      exports: run.exports,
    })),
    total: body.total,
    notExported: body.not_exported,
    nextOffset: body.next_offset ?? null,
  };
}

/** A page of the export protocol, newest first. */
export async function fetchExportProtocol(
  offset = 0,
): Promise<FinanceExportPage<ExportProtocolEntry>> {
  const body = await traegerRequest<
    BackendPage & { files: BackendEntry[] | null }
  >(pagePath(`${BASE}/files`, offset));
  return {
    items: (body.files ?? []).map(entryFromBackend),
    total: body.total,
    nextOffset: body.next_offset ?? null,
  };
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

/** What a DATEV download holds (#6433). */
export interface DatevDownloadOptions {
  /** Every booking of the run again, also those an earlier file held. */
  readonly all?: boolean;
  /** "YYYY-MM-DD", the first day of the fiscal year; the earliest if unset. */
  readonly fiscalYear?: string;
}

/**
 * Downloads the DATEV booking batch of a run with the next file number:
 * the bookings no DATEV file held yet, or with `all` every booking again.
 * A refusal keeps its code: `parent_billing.datev_not_ready` while values
 * are missing, `parent_billing.datev_bookings_invalid` with the invoices
 * DATEV would refuse, `parent_billing.datev_no_new_bookings` when every
 * booking is in a file, `parent_billing.datev_bookings_taken` when another
 * download took them a moment earlier.
 */
export async function downloadDatevBatch(
  kind: ExportRunPathKind,
  runId: string,
  options: DatevDownloadOptions = {},
): Promise<BillingFile> {
  const query = new URLSearchParams();
  if (options.all) query.set("mode", "all");
  if (options.fiscalYear) query.set("fiscal_year", options.fiscalYear);
  const search = query.size > 0 ? `?${query.toString()}` : "";
  return billingFile(
    `${BASE}/runs/${kind}/${encodeURIComponent(runId)}/datev${search}`,
    {},
    "EXTF_Buchungsstapel.csv",
  );
}

/** The DATEV bookings of a run on the export page (#6433). */
export interface DatevRunState {
  /** Bookings the next standard files hold, across fiscal years. */
  readonly newBookings: number;
  /** First days ("YYYY-MM-DD") of the fiscal years of its bookings. */
  readonly fiscalYears: readonly string[];
}

/** The DATEV states of the runs of a page, by `runFileKey`-like key. */
export interface DatevRunStates {
  /** False while the DATEV values make no valid file; then no state. */
  readonly ready: boolean;
  readonly runs: ReadonlyMap<string, DatevRunState>;
}

/** The key of a run in `DatevRunStates.runs`. */
export function datevRunKey(run: Pick<ExportRun, "runKind" | "runId">) {
  return `${run.runKind}/${run.runId}`;
}

interface BackendDatevRuns {
  ready: boolean;
  runs:
    | {
        run_kind: ExportRunKind;
        run_id: number;
        new_bookings: number;
        fiscal_years: string[] | null;
      }[]
    | null;
}

/** Runs one request asks for: the size of a page of the run list. */
const DATEV_RUNS_PER_REQUEST = 50;

async function fetchDatevRunPage(
  runs: readonly ExportRun[],
): Promise<BackendDatevRuns> {
  const query = new URLSearchParams();
  for (const kind of ["care", "holiday"] as const) {
    const ids = runs
      .filter((run) => run.runKind === kind)
      .map((run) => run.runId);
    if (ids.length > 0) query.set(kind, ids.join(","));
  }
  if (query.size === 0) return { ready: true, runs: [] };
  return traegerRequest<BackendDatevRuns>(
    `${BASE}/datev-runs?${query.toString()}`,
  );
}

/**
 * Reads the new DATEV bookings of the monthly and holiday runs given, one
 * request per page of runs.
 */
export async function fetchDatevRunStates(
  runs: readonly ExportRun[],
): Promise<DatevRunStates> {
  const pages: ExportRun[][] = [];
  for (let i = 0; i < runs.length; i += DATEV_RUNS_PER_REQUEST) {
    pages.push(runs.slice(i, i + DATEV_RUNS_PER_REQUEST));
  }
  const bodies = await Promise.all(pages.map(fetchDatevRunPage));
  return {
    ready: bodies.length > 0 && bodies.every((body) => body.ready),
    runs: new Map(
      bodies.flatMap((body) =>
        (body.runs ?? []).map(
          (run) =>
            [
              datevRunKey({ runKind: run.run_kind, runId: String(run.run_id) }),
              {
                newBookings: run.new_bookings,
                fiscalYears: run.fiscal_years ?? [],
              },
            ] as const,
        ),
      ),
    ),
  };
}

const DATEV_PROBLEM_FIELDS = new Set<string>([
  "debtor_number",
  "document_date",
  "document_number",
  "amount",
]);
const DATEV_PROBLEM_REASONS = new Set<string>([
  "missing",
  "invalid",
  "out_of_period",
]);

/**
 * The invoices a refused DATEV download names in its details; an entry of
 * another shape is left out, never guessed.
 */
export function datevBookingProblems(
  details: Record<string, unknown> | undefined,
): DatevBookingProblem[] {
  const documents = details?.documents;
  if (!Array.isArray(documents)) return [];
  return documents.flatMap((entry: unknown) => {
    if (typeof entry !== "object" || entry === null) return [];
    const { document_number, field, reason } = entry as Record<string, unknown>;
    if (
      typeof document_number !== "string" ||
      typeof field !== "string" ||
      typeof reason !== "string" ||
      !DATEV_PROBLEM_FIELDS.has(field) ||
      !DATEV_PROBLEM_REASONS.has(reason)
    ) {
      return [];
    }
    return [
      {
        documentNumber: document_number,
        field: field as DatevBookingProblem["field"],
        reason: reason as DatevBookingProblem["reason"],
      },
    ];
  });
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
