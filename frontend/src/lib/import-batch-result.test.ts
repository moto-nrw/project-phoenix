import { describe, expect, it } from "vitest";

import {
  importBatchFailureAlertType,
  importBatchFailureMessage,
  readImportBatchFailure,
} from "./import-batch-result";

const committed = {
  TotalRows: 205,
  CreatedCount: 100,
  UpdatedCount: 0,
  ErrorCount: 1,
  Errors: [{ RowNumber: 152 }],
};

describe("readImportBatchFailure", () => {
  it("reads details.result from an import_batch_failed envelope", () => {
    expect(
      readImportBatchFailure({
        status: "error",
        error: "Import fehlgeschlagen",
        code: "import_batch_failed",
        details: { result: committed },
      }),
    ).toEqual(committed);
  });

  it("ignores other error envelopes", () => {
    expect(
      readImportBatchFailure({
        error: "Import fehlgeschlagen",
        message: "Import fehlgeschlagen",
      }),
    ).toBeNull();
    expect(readImportBatchFailure({ code: "import_batch_failed" })).toBeNull();
    expect(readImportBatchFailure(null)).toBeNull();
  });
});

describe("importBatchFailureMessage", () => {
  it("names saved rows and the blocking row", () => {
    expect(importBatchFailureMessage(committed)).toBe(
      "100 Zeilen sind gespeichert. Zeile 152 hat den Rest angehalten. Bitte korrigieren Sie diese Zeile. Laden Sie die Datei danach erneut hoch. Die gespeicherten Zeilen bleiben.",
    );
    expect(importBatchFailureAlertType(committed)).toBe("warning");
  });

  it("does not claim progress when the first batch rolled back", () => {
    const firstBatch = {
      TotalRows: 40,
      CreatedCount: 0,
      UpdatedCount: 0,
      ErrorCount: 1,
      Errors: [{ RowNumber: 5 }],
    };
    expect(importBatchFailureMessage(firstBatch)).toBe(
      "Zeile 5 hat nicht geklappt. Bitte korrigieren Sie diese Zeile. Laden Sie die Datei danach erneut hoch.",
    );
    expect(importBatchFailureAlertType(firstBatch)).toBe("error");
  });
});
