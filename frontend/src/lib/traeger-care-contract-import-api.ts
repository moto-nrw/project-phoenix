/**
 * Client of the import of children and their existing care contracts
 * (#4723): the preview of a CSV file with its column mapping, the import of
 * the ready rows and the log of the imports. The file travels as multipart
 * form; the backend reads it in memory and never stores it.
 * Backend int64 IDs leave here as strings; calendar dates stay "YYYY-MM-DD".
 */

import { apiErrorFromResponse, transportFetch } from "./api-error";
import { readJson, traegerRequest } from "./traeger-api";

const IMPORTS_PATH = "/api/traeger/contract-imports";

/** Largest file the backend reads, in bytes. */
export const MAX_CONTRACT_IMPORT_FILE_BYTES = 2 * 1024 * 1024;

export type ContractImportField =
  | "child_first_name"
  | "child_last_name"
  | "child_birth_date"
  | "school"
  | "module"
  | "weekdays"
  | "effective_from"
  | "ends_on"
  | "concluded_on"
  | "party_first_name"
  | "party_last_name"
  | "party_email"
  | "debtor_number";

export type ContractImportRowStatus =
  "ready" | "duplicate" | "conflict" | "invalid" | "imported";

/** What the import does with the debtor number of a row; "" without one. */
type ContractDebtorNumberStatus =
  "" | "new" | "same" | "differs" | "rows_differ" | "invalid" | "no_family";

/** Header of the column each field comes from; "" leaves it unmapped. */
type ContractImportColumns = Readonly<Record<ContractImportField, string>>;

export interface ContractImportMapping {
  readonly columns: ContractImportColumns;
  /** The school of every row of a file without a school column; "" with one. */
  readonly schoolId: string;
  /** The module of every row of a file without a module column; "" with one. */
  readonly moduleName: string;
}

interface ContractImportProblem {
  readonly field: ContractImportField;
  readonly code: string;
}

export interface ContractImportRow {
  readonly line: number;
  readonly status: ContractImportRowStatus;
  readonly childFirstName: string;
  readonly childLastName: string;
  readonly childBirthDate: string | null;
  readonly schoolName: string;
  readonly moduleNames: readonly string[];
  readonly weekdays: readonly string[];
  readonly effectiveFrom: string | null;
  readonly endsOn: string | null;
  readonly partyName: string;
  readonly partyEmail: string;
  readonly problems: readonly ContractImportProblem[];
  /** Notes that do not hold the row back: "start_moved", "imported_before". */
  readonly hints: readonly string[];
  /** The stored contract, or the running one a duplicate row found. */
  readonly contractId: string | null;
  /** The line that names the same child before this row. */
  readonly repeatsLine: number | null;
  readonly debtorNumber: string;
  readonly debtorNumberStatus: ContractDebtorNumberStatus;
}

interface ContractImportCounts {
  readonly ready: number;
  readonly duplicate: number;
  readonly conflict: number;
  readonly invalid: number;
  readonly imported: number;
  readonly debtorNumbers: number;
  readonly debtorConflicts: number;
}

interface ContractImportModule {
  readonly name: string;
  readonly phaseName: string;
  readonly hasConditions: boolean;
}

export interface ContractImportSchool {
  readonly id: string;
  readonly name: string;
  readonly modules: readonly ContractImportModule[];
}

export interface ContractImportResult {
  readonly encoding: string;
  readonly delimiter: string;
  readonly headers: readonly string[];
  readonly mapping: ContractImportMapping;
  /** Required fields without a column or a choice. */
  readonly unmapped: readonly ContractImportField[];
  readonly schools: readonly ContractImportSchool[];
  /** Whom a debtor number of the Träger names. */
  readonly debtorNumberScope: "payer" | "child";
  readonly fingerprint: string;
  readonly applied: boolean;
  readonly counts: ContractImportCounts;
  readonly rows: readonly ContractImportRow[];
}

export interface ContractImportLogEntry {
  readonly file: string;
  readonly accountId: string | null;
  readonly importedAt: string;
  readonly contracts: number;
}

export interface ContractImportRequest {
  readonly file: File;
  /** null takes the mapping moto suggests from the headers. */
  readonly mapping: ContractImportMapping | null;
  /** The fingerprint of the preview the office confirmed; only for the import. */
  readonly fingerprint?: string;
}

interface BackendMapping {
  columns: Record<ContractImportField, string>;
  school_id: number;
  module_name: string;
}

interface BackendRow {
  line: number;
  status: ContractImportRowStatus;
  child_first_name: string;
  child_last_name: string;
  child_birth_date: string | null;
  school_name: string;
  module_names: string[] | null;
  weekdays: string[] | null;
  effective_from: string | null;
  ends_on: string | null;
  party_name: string;
  party_email: string;
  problems: ContractImportProblem[] | null;
  hints?: string[] | null;
  contract_id?: number;
  repeats_line?: number;
  debtor_number?: string;
  debtor_number_status?: ContractDebtorNumberStatus;
}

interface BackendResult {
  encoding: string;
  delimiter: string;
  headers: string[] | null;
  mapping: BackendMapping;
  unmapped: ContractImportField[] | null;
  schools:
    | {
        id: number;
        name: string;
        modules:
          | { name: string; phase_name: string; has_conditions: boolean }[]
          | null;
      }[]
    | null;
  debtor_number_scope: "payer" | "child";
  fingerprint?: string;
  applied: boolean;
  counts: {
    ready: number;
    duplicate: number;
    conflict: number;
    invalid: number;
    imported: number;
    debtor_numbers?: number;
    debtor_conflicts?: number;
  };
  rows: BackendRow[] | null;
}

interface BackendLogEntry {
  file: string;
  account_id: number | null;
  imported_at: string;
  contracts: number;
}

function toBackendMapping(mapping: ContractImportMapping): BackendMapping {
  return {
    columns: { ...mapping.columns },
    school_id: mapping.schoolId === "" ? 0 : Number(mapping.schoolId),
    module_name: mapping.moduleName,
  };
}

function mapRow(row: BackendRow): ContractImportRow {
  return {
    line: row.line,
    status: row.status,
    childFirstName: row.child_first_name,
    childLastName: row.child_last_name,
    childBirthDate: row.child_birth_date,
    schoolName: row.school_name,
    moduleNames: row.module_names ?? [],
    weekdays: row.weekdays ?? [],
    effectiveFrom: row.effective_from,
    endsOn: row.ends_on,
    partyName: row.party_name,
    partyEmail: row.party_email,
    problems: row.problems ?? [],
    hints: row.hints ?? [],
    contractId: row.contract_id ? String(row.contract_id) : null,
    repeatsLine: row.repeats_line ?? null,
    debtorNumber: row.debtor_number ?? "",
    debtorNumberStatus: row.debtor_number_status ?? "",
  };
}

function mapImportResult(result: BackendResult): ContractImportResult {
  return {
    encoding: result.encoding,
    delimiter: result.delimiter,
    headers: result.headers ?? [],
    mapping: {
      columns: result.mapping.columns,
      schoolId: result.mapping.school_id
        ? String(result.mapping.school_id)
        : "",
      moduleName: result.mapping.module_name ?? "",
    },
    unmapped: result.unmapped ?? [],
    schools: (result.schools ?? []).map((school) => ({
      id: String(school.id),
      name: school.name,
      modules: (school.modules ?? []).map((entry) => ({
        name: entry.name,
        phaseName: entry.phase_name,
        hasConditions: entry.has_conditions,
      })),
    })),
    debtorNumberScope: result.debtor_number_scope,
    fingerprint: result.fingerprint ?? "",
    applied: result.applied,
    counts: {
      ready: result.counts.ready,
      duplicate: result.counts.duplicate,
      conflict: result.counts.conflict,
      invalid: result.counts.invalid,
      imported: result.counts.imported,
      debtorNumbers: result.counts.debtor_numbers ?? 0,
      debtorConflicts: result.counts.debtor_conflicts ?? 0,
    },
    rows: (result.rows ?? []).map(mapRow),
  };
}

async function postImport(
  path: string,
  request: ContractImportRequest,
): Promise<ContractImportResult> {
  const form = new FormData();
  form.append("file", request.file);
  if (request.mapping) {
    form.append("mapping", JSON.stringify(toBackendMapping(request.mapping)));
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
export function previewContractImport(
  request: ContractImportRequest,
): Promise<ContractImportResult> {
  return postImport(`${IMPORTS_PATH}/preview`, request);
}

/** Takes over the ready rows. */
export function applyContractImport(
  request: ContractImportRequest,
): Promise<ContractImportResult> {
  return postImport(IMPORTS_PATH, request);
}

/** The imports of the Träger, newest first. */
export async function fetchContractImportLog(): Promise<
  ContractImportLogEntry[]
> {
  const entries = await traegerRequest<BackendLogEntry[] | null>(IMPORTS_PATH);
  return (entries ?? []).map((entry) => ({
    file: entry.file,
    accountId: entry.account_id === null ? null : String(entry.account_id),
    importedAt: entry.imported_at,
    contracts: entry.contracts,
  }));
}
