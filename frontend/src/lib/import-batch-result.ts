const importBatchFailedCode = "import_batch_failed";
const abortingWriteCodes = new Set(["creation_failed", "update_failed"]);

interface ImportBatchRowError {
  RowNumber: number;
  Errors?: ReadonlyArray<{ code?: string }>;
}

export interface ImportBatchResult<T = ImportBatchRowError> {
  TotalRows: number;
  CreatedCount: number;
  UpdatedCount: number;
  ErrorCount: number;
  Errors: T[];
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === "object";
}

function importBatchErrors(value: unknown): unknown[] | undefined {
  if (value == null) return [];
  if (Array.isArray(value)) return value;
  return undefined;
}

function rowHasAbortingWrite(entry: ImportBatchRowError): boolean {
  return (entry.Errors ?? []).some((error) =>
    abortingWriteCodes.has(error.code ?? ""),
  );
}

/**
 * Reads the committed import outcome from an `import_batch_failed` envelope.
 * Returns null for any other error so callers keep their generic failure path.
 * A nil Go slice encodes as `Errors: null`; that is an empty list, not a
 * malformed payload.
 */
export function readImportBatchFailure<T>(
  payload: unknown,
): ImportBatchResult<T> | null {
  if (!isRecord(payload) || payload.code !== importBatchFailedCode) {
    return null;
  }
  if (!isRecord(payload.details) || !isRecord(payload.details.result)) {
    return null;
  }
  const result = payload.details.result;
  const errors = importBatchErrors(result.Errors);
  if (
    typeof result.CreatedCount !== "number" ||
    typeof result.UpdatedCount !== "number" ||
    typeof result.ErrorCount !== "number" ||
    typeof result.TotalRows !== "number" ||
    errors === undefined
  ) {
    return null;
  }
  return { ...result, Errors: errors } as unknown as ImportBatchResult<T>;
}

/**
 * Reads the refusal that stopped a batch, such as a full Kinderkontingent
 * (#3571): `{ code, details }` in `details.rejection` of an
 * `import_batch_failed` envelope. Null when a batch failed for another reason.
 */
export function readImportBatchRejection(payload: unknown): unknown {
  if (!isRecord(payload) || payload.code !== importBatchFailedCode) {
    return null;
  }
  if (!isRecord(payload.details) || !isRecord(payload.details.rejection)) {
    return null;
  }
  return payload.details.rejection;
}

export function importBatchSavedCount(result: {
  CreatedCount: number;
  UpdatedCount: number;
}): number {
  return result.CreatedCount + result.UpdatedCount;
}

/** Create-mode preview records already-written rows as already_exists, not as remaining work. */
export function countAlreadyExistsRows(
  rows:
    | ReadonlyArray<{ Errors: ReadonlyArray<{ code: string }> }>
    | null
    | undefined,
): number {
  return (rows ?? []).filter((row) =>
    row.Errors.some((error) => error.code === "already_exists"),
  ).length;
}

function importBatchBlockingRow(result: ImportBatchResult): number | undefined {
  for (let index = result.Errors.length - 1; index >= 0; index -= 1) {
    const row = result.Errors[index];
    if (row && row.RowNumber > 0 && rowHasAbortingWrite(row)) {
      return row.RowNumber;
    }
  }
  return undefined;
}

function savedRowsSentence(saved: number): string {
  return saved === 1
    ? "1 Zeile ist gespeichert."
    : `${saved} Zeilen sind gespeichert.`;
}

/**
 * Persistent alert copy: saved rows first, then the blocking row and next
 * step. A refusal (already worded for the reader) replaces the blocking row:
 * the batch stopped as a whole, not at one row.
 */
export function importBatchFailureMessage(
  result: ImportBatchResult,
  refusal?: string | null,
): string {
  const saved = importBatchSavedCount(result);
  if (refusal) {
    if (saved === 0) return refusal;
    return `${savedRowsSentence(saved)} ${refusal} Die gespeicherten Zeilen bleiben.`;
  }
  const blockingRow = importBatchBlockingRow(result);
  if (saved === 0) {
    if (blockingRow === undefined) {
      return "Der Import hat nicht geklappt. Bitte prüfen Sie die Datei. Laden Sie sie danach erneut hoch.";
    }
    return `Zeile ${blockingRow} hat nicht geklappt. Bitte korrigieren Sie diese Zeile. Laden Sie die Datei danach erneut hoch.`;
  }
  const savedSentence = savedRowsSentence(saved);
  if (blockingRow === undefined) {
    return `${savedSentence} Der Rest hat nicht geklappt. Laden Sie die Datei danach erneut hoch. Die gespeicherten Zeilen bleiben.`;
  }
  return `${savedSentence} Zeile ${blockingRow} hat den Rest angehalten. Bitte korrigieren Sie diese Zeile. Laden Sie die Datei danach erneut hoch. Die gespeicherten Zeilen bleiben.`;
}

export function importBatchFailureAlertType(
  result: ImportBatchResult,
): "warning" | "error" {
  return importBatchSavedCount(result) > 0 ? "warning" : "error";
}
