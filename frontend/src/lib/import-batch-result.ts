const importBatchFailedCode = "import_batch_failed";

interface ImportBatchRowError {
  RowNumber: number;
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

/**
 * Reads the committed import outcome from an `import_batch_failed` envelope.
 * Returns null for any other error so callers keep their generic failure path.
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
  if (
    typeof result.CreatedCount !== "number" ||
    typeof result.UpdatedCount !== "number" ||
    typeof result.ErrorCount !== "number" ||
    typeof result.TotalRows !== "number" ||
    !Array.isArray(result.Errors)
  ) {
    return null;
  }
  return result as unknown as ImportBatchResult<T>;
}

export function importBatchSavedCount(result: {
  CreatedCount: number;
  UpdatedCount: number;
}): number {
  return result.CreatedCount + result.UpdatedCount;
}

function importBatchBlockingRow(result: ImportBatchResult): number | undefined {
  const row = result.Errors.find((entry) => entry.RowNumber > 0);
  return row?.RowNumber;
}

/** Persistent alert copy: saved rows first, then the blocking row and next step. */
export function importBatchFailureMessage(result: ImportBatchResult): string {
  const saved = importBatchSavedCount(result);
  const blockingRow = importBatchBlockingRow(result);
  if (saved === 0) {
    if (blockingRow === undefined) {
      return "Der Import hat nicht geklappt. Bitte prüfen Sie die Datei. Laden Sie sie danach erneut hoch.";
    }
    return `Zeile ${blockingRow} hat nicht geklappt. Bitte korrigieren Sie diese Zeile. Laden Sie die Datei danach erneut hoch.`;
  }
  const savedSentence =
    saved === 1
      ? "1 Zeile ist gespeichert."
      : `${saved} Zeilen sind gespeichert.`;
  const blockingSentence =
    blockingRow === undefined
      ? "Eine Zeile hat den Rest angehalten."
      : `Zeile ${blockingRow} hat den Rest angehalten.`;
  return `${savedSentence} ${blockingSentence} Bitte korrigieren Sie diese Zeile. Laden Sie die Datei danach erneut hoch. Die gespeicherten Zeilen bleiben.`;
}

export function importBatchFailureAlertType(
  result: ImportBatchResult,
): "warning" | "error" {
  return importBatchSavedCount(result) > 0 ? "warning" : "error";
}
