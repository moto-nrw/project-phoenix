/**
 * Client of the import of existing SEPA mandates (#6198): the preview of a
 * CSV file with its column mapping, the import of the ready rows and the log
 * of the imports. The file travels as multipart form; the backend reads it
 * in memory and never stores it. No answer carries an IBAN: a valid one
 * comes back masked, an invalid one not at all.
 * Backend int64 IDs leave here as strings; calendar dates stay "YYYY-MM-DD".
 */

import { apiErrorFromResponse, transportFetch } from "./api-error";
import type { MandateSequence } from "./traeger-sepa-api";
import { readJson, traegerRequest } from "./traeger-api";

const IMPORTS_PATH = "/api/traeger/sepa-mandates/imports";

/** Largest file the backend reads, in bytes. */
export const MAX_IMPORT_FILE_BYTES = 2 * 1024 * 1024;

export type ImportField =
  | "reference"
  | "signed_on"
  | "iban"
  | "bic"
  | "account_holder"
  | "payer"
  | "creditor_id"
  | "sequence"
  | "last_collected_on"
  | "debtor_number";

export type PayerKey = "email" | "contract_id";

export type ImportRowStatus =
  "ready" | "clarify" | "duplicate" | "conflict" | "invalid" | "imported";

/** Header of the column each field comes from; "" leaves it unmapped. */
type ImportColumns = Readonly<Record<ImportField, string>>;

export interface ImportMapping {
  readonly columns: ImportColumns;
  readonly payerKey: PayerKey;
  /** The office says every mandate of the file is given to its creditor ID. */
  readonly creditorFromTraeger: boolean;
  /** The creditor ID the office saw when it confirmed; "" without confirmation. */
  readonly confirmedCreditorId: string;
}

export interface ImportProblem {
  readonly field: ImportField;
  readonly code: string;
}

export interface ImportRow {
  readonly line: number;
  readonly status: ImportRowStatus;
  readonly reference: string;
  readonly signedOn: string | null;
  readonly ibanMasked: string;
  readonly bic: string;
  readonly accountHolder: string;
  readonly contractId: string | null;
  readonly payerFirstName: string;
  readonly payerLastName: string;
  readonly sequence: MandateSequence | "";
  readonly lastCollectedOn: string | null;
  readonly mandateId: string | null;
  readonly problems: readonly ImportProblem[];
  readonly hints: readonly string[];
  /** Debtor number of the row for the family (#4693), "" without the column. */
  readonly debtorNumber: string;
  readonly debtorNumberStatus: DebtorNumberStatus | "";
}

/** What the import does with the debtor number of a row (#4693). */
type DebtorNumberStatus =
  "new" | "same" | "siblings_differ" | "differs" | "invalid" | "no_family";

type ImportCounts = Readonly<
  Record<ImportRowStatus, number> & {
    debtorNumbers: number;
    debtorConflicts: number;
    /** Rows without IBAN and reference the import skipped (#7009). */
    withoutMandate: number;
  }
>;

export interface ImportResult {
  readonly encoding: string;
  readonly delimiter: string;
  readonly headers: readonly string[];
  readonly mapping: ImportMapping;
  /** Required fields still without a column; rows are read once it is empty. */
  readonly unmapped: readonly ImportField[];
  readonly creditorId: string;
  /** Names the rows and what the import does with each, as the preview showed them. */
  readonly fingerprint: string;
  readonly applied: boolean;
  readonly counts: ImportCounts;
  readonly rows: readonly ImportRow[];
}

export interface ImportLogEntry {
  readonly file: string;
  readonly accountId: string | null;
  readonly importedAt: string;
  readonly mandates: number;
}

export interface ImportRequest {
  readonly file: File;
  /** null lets moto suggest the mapping from the headers. */
  readonly mapping: ImportMapping | null;
  /** Answer per line of a row to clarify. */
  readonly sequences?: Readonly<Record<number, MandateSequence>>;
  /** The fingerprint of the preview the office confirmed; the import takes over only that. */
  readonly fingerprint?: string;
}

interface BackendMapping {
  columns: Record<ImportField, string>;
  payer_key: PayerKey;
  creditor_from_traeger: boolean;
  confirmed_creditor_id?: string;
}

interface BackendRow {
  line: number;
  status: ImportRowStatus;
  reference: string;
  signed_on: string | null;
  iban_masked: string;
  bic: string;
  account_holder: string;
  contract_id?: number;
  payer_first_name: string;
  payer_last_name: string;
  sequence: MandateSequence | "";
  last_collected_on: string | null;
  mandate_id?: number;
  problems: ImportProblem[] | null;
  hints: string[] | null;
  debtor_number?: string;
  debtor_number_status?: DebtorNumberStatus;
}

interface BackendResult {
  encoding: string;
  delimiter: string;
  headers: string[] | null;
  mapping: BackendMapping;
  unmapped: ImportField[] | null;
  creditor_id: string;
  fingerprint?: string;
  applied: boolean;
  counts: Record<ImportRowStatus, number> & {
    debtor_numbers?: number;
    debtor_conflicts?: number;
    without_mandate?: number;
  };
  rows: BackendRow[] | null;
}

interface BackendLogEntry {
  file: string;
  account_id: number | null;
  imported_at: string;
  mandates: number;
}

function toBackendMapping(mapping: ImportMapping): BackendMapping {
  return {
    columns: { ...mapping.columns },
    payer_key: mapping.payerKey,
    creditor_from_traeger: mapping.creditorFromTraeger,
    confirmed_creditor_id: mapping.confirmedCreditorId,
  };
}

function mapRow(row: BackendRow): ImportRow {
  return {
    line: row.line,
    status: row.status,
    reference: row.reference,
    signedOn: row.signed_on,
    ibanMasked: row.iban_masked,
    bic: row.bic,
    accountHolder: row.account_holder,
    contractId: row.contract_id ? String(row.contract_id) : null,
    payerFirstName: row.payer_first_name,
    payerLastName: row.payer_last_name,
    sequence: row.sequence,
    lastCollectedOn: row.last_collected_on,
    mandateId: row.mandate_id ? String(row.mandate_id) : null,
    problems: row.problems ?? [],
    hints: row.hints ?? [],
    debtorNumber: row.debtor_number ?? "",
    debtorNumberStatus: row.debtor_number_status ?? "",
  };
}

function mapImportResult(result: BackendResult): ImportResult {
  return {
    encoding: result.encoding,
    delimiter: result.delimiter,
    headers: result.headers ?? [],
    mapping: {
      columns: result.mapping.columns,
      payerKey: result.mapping.payer_key,
      creditorFromTraeger: result.mapping.creditor_from_traeger,
      confirmedCreditorId: result.mapping.confirmed_creditor_id ?? "",
    },
    unmapped: result.unmapped ?? [],
    creditorId: result.creditor_id,
    fingerprint: result.fingerprint ?? "",
    applied: result.applied,
    counts: {
      ...result.counts,
      debtorNumbers: result.counts.debtor_numbers ?? 0,
      debtorConflicts: result.counts.debtor_conflicts ?? 0,
      withoutMandate: result.counts.without_mandate ?? 0,
    },
    rows: (result.rows ?? []).map(mapRow),
  };
}

async function postImport(
  path: string,
  request: ImportRequest,
): Promise<ImportResult> {
  const form = new FormData();
  form.append("file", request.file);
  if (request.mapping) {
    form.append("mapping", JSON.stringify(toBackendMapping(request.mapping)));
  }
  if (request.sequences && Object.keys(request.sequences).length > 0) {
    form.append("sequences", JSON.stringify(request.sequences));
  }
  if (request.fingerprint) {
    form.append("fingerprint", request.fingerprint);
  }
  const response = await transportFetch(path, {
    method: "POST",
    credentials: "include",
    cache: "no-store",
    body: form,
  });
  if (!response.ok) {
    throw await apiErrorFromResponse(
      response,
      `Request failed (${response.status})`,
    );
  }
  const data = await readJson(response);
  return mapImportResult((data as { data: BackendResult }).data);
}

/** Reads the file and answers every row; stores nothing. */
export function previewSepaImport(
  request: ImportRequest,
): Promise<ImportResult> {
  return postImport(`${IMPORTS_PATH}/preview`, request);
}

/** Takes over the ready rows and the clarified ones. */
export function applySepaImport(request: ImportRequest): Promise<ImportResult> {
  return postImport(IMPORTS_PATH, request);
}

/** The imports of the Träger, newest first. */
export async function fetchSepaImportLog(): Promise<ImportLogEntry[]> {
  const entries = await traegerRequest<BackendLogEntry[] | null>(IMPORTS_PATH);
  return (entries ?? []).map((entry) => ({
    file: entry.file,
    accountId: entry.account_id === null ? null : String(entry.account_id),
    importedAt: entry.imported_at,
    mandates: entry.mandates,
  }));
}
