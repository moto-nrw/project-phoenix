import { describe, expect, it } from "vitest";

import { suggestExpiry } from "./staff-qualification-api";
import { qualificationsCsv } from "./staff-qualification-csv";

describe("suggestExpiry", () => {
  it("adds the validity in months to the acquisition day", () => {
    expect(suggestExpiry("2026-03-10", 24)).toBe("2028-03-10");
    expect(suggestExpiry("2026-11-15", 3)).toBe("2027-02-15");
  });

  it("moves a missing month day to the end of the month", () => {
    expect(suggestExpiry("2026-01-31", 1)).toBe("2026-02-28");
    expect(suggestExpiry("2027-08-31", 6)).toBe("2028-02-29");
  });

  it("suggests nothing without validity or a valid day", () => {
    expect(suggestExpiry("2026-03-10", null)).toBeNull();
    expect(suggestExpiry("10.03.2026", 12)).toBeNull();
  });
});

describe("qualificationsCsv", () => {
  it("writes a semicolon file Excel opens with Umlauts", () => {
    const csv = qualificationsCsv([
      {
        staffId: "1",
        qualificationId: "9",
        typeId: "2",
        name: "Erste Hilfe am Kind",
        acquiredOn: "2024-05-02",
        expiresOn: "2026-05-02",
        status: "expiring_soon",
        mandatory: false,
        firstName: "Jörg",
        lastName: "Müller; Sohn",
      },
      {
        staffId: "2",
        qualificationId: null,
        typeId: "3",
        name: "Erweitertes Führungszeugnis",
        acquiredOn: null,
        expiresOn: null,
        status: "missing",
        mandatory: true,
        firstName: "Ana",
        lastName: "Silva",
      },
    ]);
    expect(
      csv.startsWith("﻿Name;Nachweis;Erworben;Gültig bis;Status;Pflicht\r\n"),
    ).toBe(true);
    expect(csv).toContain(
      '"Jörg Müller; Sohn";Erste Hilfe am Kind;02.05.2024;02.05.2026;Läuft bald ab;Nein',
    );
    expect(csv).toContain("Ana Silva;Erweitertes Führungszeugnis;;;Fehlt;Ja");
  });

  it("keeps a name Excel would read as a formula as text", () => {
    const csv = qualificationsCsv([
      {
        staffId: "1",
        qualificationId: "9",
        typeId: null,
        name: '=HYPERLINK("http://x")',
        acquiredOn: null,
        expiresOn: null,
        status: "valid",
        mandatory: false,
        firstName: "Max",
        lastName: "Muster",
      },
    ]);
    expect(csv).toContain(`Max Muster;"'=HYPERLINK(""http://x"")";`);
  });
});
