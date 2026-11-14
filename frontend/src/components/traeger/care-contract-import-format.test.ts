import { describe, expect, it } from "vitest";
import type { ContractImportRow } from "~/lib/traeger-care-contract-import-api";
import { formatConclusion } from "./care-contract-format";
import {
  contractImportDebtorText,
  contractImportPeriod,
  contractImportProblemText,
  contractImportWeekdays,
  matchesContractImportFilter,
} from "./care-contract-import-format";

const row: ContractImportRow = {
  line: 2,
  status: "ready",
  childFirstName: "Mia",
  childLastName: "Beispiel",
  childBirthDate: null,
  schoolName: "Grundschule Nord",
  moduleNames: ["OGS bis 16 Uhr"],
  weekdays: ["mon"],
  effectiveFrom: "2026-08-01",
  endsOn: null,
  partyName: "Eva Beispiel",
  partyEmail: "",
  problems: [],
  contractId: null,
  repeatsLine: null,
  debtorNumber: "",
  debtorNumberStatus: "",
};

describe("care contract import format", () => {
  it("writes three and more days in a row as a span", () => {
    expect(contractImportWeekdays(["mon", "tue", "wed", "thu", "fri"])).toBe(
      "Mo–Fr",
    );
    expect(contractImportWeekdays(["mon", "tue", "thu"])).toBe("Mo, Di, Do");
    expect(contractImportWeekdays(["fri", "mon", "tue", "wed"])).toBe(
      "Mo–Mi, Fr",
    );
    expect(contractImportWeekdays([])).toBe("–");
  });

  it("names the period with or without an end", () => {
    expect(contractImportPeriod(row)).toBe("ab 01.08.2026");
    expect(contractImportPeriod({ ...row, endsOn: "2027-07-31" })).toBe(
      "01.08.2026 bis 31.07.2027",
    );
    expect(contractImportPeriod({ ...row, effectiveFrom: null })).toBe("–");
  });

  it("says what holds a row back and what happens with its number", () => {
    expect(
      contractImportProblemText({ field: "module", code: "unknown" }, row),
    ).toBe("Dieses Modul gibt es an der Schule nicht.");
    expect(
      contractImportProblemText(
        { field: "child_last_name", code: "repeated" },
        { ...row, repeatsLine: 4 },
      ),
    ).toBe("Das Kind steht schon in Zeile 4.");
    expect(
      contractImportProblemText({ field: "ends_on", code: "new_code" }, row),
    ).toBe("Vertragsende prüfen.");
    expect(
      contractImportDebtorText({
        ...row,
        debtorNumber: "70001",
        debtorNumberStatus: "new",
      }),
    ).toBe("Debitorennummer 70001 wird eingetragen.");
    expect(contractImportDebtorText(row)).toBeNull();
  });

  it("shows the conclusion of an imported paper contract as a day", () => {
    expect(
      formatConclusion({ method: "papier", at: "2025-07-31T22:00:00Z" }),
    ).toBe("Am 01.08.2025, mit dem unterschriebenen Vertrag auf Papier");
    expect(
      formatConclusion({ method: "papier", at: "2026-11-06T15:12:00Z" }),
    ).toBe(
      "Am 06.11.2026 um 16:12 Uhr, mit dem unterschriebenen Vertrag auf Papier",
    );
  });

  it("sorts rows into the filters", () => {
    expect(matchesContractImportFilter(row, "ready")).toBe(true);
    expect(
      matchesContractImportFilter({ ...row, status: "conflict" }, "problems"),
    ).toBe(true);
    expect(
      matchesContractImportFilter({ ...row, status: "duplicate" }, "ready"),
    ).toBe(false);
    expect(
      matchesContractImportFilter({ ...row, status: "duplicate" }, "duplicate"),
    ).toBe(true);
  });
});
