import { describe, expect, it } from "vitest";

import {
  countAlreadyExistsRows,
  importBatchFailureAlertType,
  importBatchFailureMessage,
  readImportBatchFailure,
} from "./import-batch-result";

const abortingWrite = {
  RowNumber: 152,
  Errors: [{ code: "creation_failed" }],
};

const committed = {
  TotalRows: 205,
  CreatedCount: 100,
  UpdatedCount: 0,
  ErrorCount: 1,
  Errors: [abortingWrite],
};

function envelope(result: Record<string, unknown>) {
  return {
    status: "error",
    error: "Import fehlgeschlagen",
    code: "import_batch_failed",
    details: { result },
  };
}

describe("readImportBatchFailure", () => {
  it("reads details.result from an import_batch_failed envelope", () => {
    expect(readImportBatchFailure(envelope(committed))).toEqual(committed);
  });

  it("treats a nil Go Errors slice as an empty list", () => {
    expect(
      readImportBatchFailure(
        envelope({
          TotalRows: 205,
          CreatedCount: 100,
          UpdatedCount: 0,
          ErrorCount: 0,
          Errors: null,
        }),
      ),
    ).toEqual({
      TotalRows: 205,
      CreatedCount: 100,
      UpdatedCount: 0,
      ErrorCount: 0,
      Errors: [],
    });
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
  it("names saved rows and the aborting write, not an earlier skip", () => {
    const mixed = {
      TotalRows: 205,
      CreatedCount: 100,
      UpdatedCount: 0,
      ErrorCount: 2,
      Errors: [
        {
          RowNumber: 2,
          Errors: [{ code: "duplicate_check_failed" }],
        },
        abortingWrite,
      ],
    };
    expect(importBatchFailureMessage(mixed)).toBe(
      "100 Zeilen sind gespeichert. Zeile 152 hat den Rest angehalten. Bitte korrigieren Sie diese Zeile. Laden Sie die Datei danach erneut hoch. Die gespeicherten Zeilen bleiben.",
    );
    expect(importBatchFailureAlertType(mixed)).toBe("warning");
    expect(
      importBatchFailureMessage({
        ...mixed,
        Errors: [
          mixed.Errors[0]!,
          { RowNumber: 152, Errors: [{ code: "update_failed" }] },
        ],
      }),
    ).toContain("Zeile 152 hat den Rest angehalten");
  });

  it("does not name a row when the later batch failed without a write error", () => {
    expect(
      importBatchFailureMessage({
        TotalRows: 205,
        CreatedCount: 100,
        UpdatedCount: 0,
        ErrorCount: 0,
        Errors: [],
      }),
    ).toBe(
      "100 Zeilen sind gespeichert. Der Rest hat nicht geklappt. Laden Sie die Datei danach erneut hoch. Die gespeicherten Zeilen bleiben.",
    );
  });

  it("does not claim progress when the first batch rolled back", () => {
    const firstBatch = {
      TotalRows: 40,
      CreatedCount: 0,
      UpdatedCount: 0,
      ErrorCount: 1,
      Errors: [{ RowNumber: 5, Errors: [{ code: "creation_failed" }] }],
    };
    expect(importBatchFailureMessage(firstBatch)).toBe(
      "Zeile 5 hat nicht geklappt. Bitte korrigieren Sie diese Zeile. Laden Sie die Datei danach erneut hoch.",
    );
    expect(importBatchFailureAlertType(firstBatch)).toBe("error");
  });
});

describe("countAlreadyExistsRows", () => {
  it("counts already_exists rows and ignores other codes", () => {
    expect(
      countAlreadyExistsRows([
        { Errors: [{ code: "already_exists" }] },
        { Errors: [{ code: "already_exists" }, { code: "will_update" }] },
        { Errors: [{ code: "creation_failed" }] },
        { Errors: [] },
      ]),
    ).toBe(2);
    expect(countAlreadyExistsRows(null)).toBe(0);
  });
});
