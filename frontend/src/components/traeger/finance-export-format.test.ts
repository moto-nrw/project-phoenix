import { describe, expect, it } from "vitest";
import type { SollMapping } from "~/lib/traeger-finance-exports-api";
import { byLabel, sollFileName, sollPreview } from "./finance-export-format";
import { sollMappingErrors } from "./soll-mapping-editor";

const MAPPING: SollMapping = {
  columns: [
    { field: "debtor_number", header: "Debitor", value: "" },
    { field: "amount", header: "Betrag", value: "" },
    { field: "invoice_date", header: "Datum", value: "" },
    { field: "constant", header: "Konto", value: "4711" },
    { field: "file_number", header: "Datei", value: "" },
  ],
  delimiter: ";",
  decimalSeparator: ",",
  dateFormat: "dd.mm.yyyy",
  encoding: "utf-8-bom",
  headerRow: true,
  fileNamePattern: "csv-export-bewegungsdaten-{NR}.csv",
};

describe("finance export format", () => {
  it("previews the file as the mapping writes it", () => {
    expect(sollPreview(MAPPING, 110)).toEqual([
      "Debitor;Betrag;Datum;Konto;Datei",
      "10 4711;78,00;30.10.2026;4711;110",
      "10 4711;-78,00;02.11.2026;4711;110",
    ]);
    expect(
      sollPreview(
        {
          ...MAPPING,
          delimiter: ",",
          decimalSeparator: ".",
          dateFormat: "yyyy-mm-dd",
          headerRow: false,
        },
        1,
      ),
    ).toEqual([
      "10 4711,78.00,2026-10-30,4711,1",
      "10 4711,-78.00,2026-11-02,4711,1",
    ]);
  });

  it("previews one row per account line once the mapping names an account", () => {
    expect(
      sollPreview(
        {
          ...MAPPING,
          columns: [
            { field: "invoice_number", header: "Nr", value: "" },
            { field: "amount", header: "Betrag", value: "" },
            { field: "revenue_account", header: "Konto", value: "" },
            { field: "cost_center", header: "KOST1", value: "" },
            { field: "cost_center_2", header: "KOST2", value: "" },
          ],
        },
        110,
      ),
    ).toEqual([
      "Nr;Betrag;Konto;KOST1;KOST2",
      "MB-2026-0110;32,50;4410;KST100;FRUEH",
      "MB-2026-0110;45,50;4400;KST100;",
      "MB-2026-0111;-32,50;4410;KST100;FRUEH",
      "MB-2026-0111;-45,50;4400;KST100;",
    ]);
  });

  it("names the file and who downloaded it", () => {
    expect(sollFileName(MAPPING.fileNamePattern, 110)).toBe(
      "csv-export-bewegungsdaten-110.csv",
    );
    expect(byLabel(true, true)).toBe("von Ihnen");
    expect(byLabel(false, true)).toBe("von einem anderen Konto");
    expect(byLabel(false, false)).toBe("von einem gelöschten Konto");
  });

  it("refuses what the backend would refuse, at the field", () => {
    expect(sollMappingErrors(MAPPING)).toEqual({});
    expect(
      sollMappingErrors({
        ...MAPPING,
        columns: [{ field: "amount", header: " ", value: "" }],
        delimiter: ",",
        decimalSeparator: ",",
        fileNamePattern: "../sollstellung.csv",
      }),
    ).toEqual({
      "columns.0.header": "Bitte einen Spaltennamen eingeben.",
      decimal_separator:
        "Das Dezimalzeichen muss sich vom Trennzeichen unterscheiden.",
      file_name_pattern:
        "Der Name braucht einmal {NR} und nur Buchstaben, Ziffern, Punkt, Strich und Unterstrich.",
    });
    expect(sollMappingErrors({ ...MAPPING, columns: [] })).toHaveProperty(
      "columns",
    );
  });
});
