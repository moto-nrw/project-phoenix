import { afterAll, beforeAll, describe, expect, it, vi } from "vitest";

import { schoolPath } from "./school-url";

// schoolPath entscheidet anhand des Hosts und merkt sich das Ergebnis. Statt
// window zu ersetzen, wird der erwartete Host auf den der Testumgebung
// gesetzt — damit gilt für diese Datei durchgehend "auf dem Schul-Host".
beforeAll(() => {
  vi.stubEnv("NEXT_PUBLIC_SCHOOL_HOSTNAME", window.location.host);
});

afterAll(() => {
  vi.unstubAllEnvs();
});

describe("schoolPath auf dem Schul-Host", () => {
  it("schneidet das /school-Präfix ab", () => {
    expect(schoolPath("/school/aufsichten")).toBe("/aufsichten");
    expect(schoolPath("/school/klasse/Klasse%202a")).toBe(
      "/klasse/Klasse%202a",
    );
  });

  it("macht aus der Portal-Wurzel die Wurzel", () => {
    expect(schoolPath("/school")).toBe("/");
  });

  it("behält die Wurzel, wenn nur ein Query folgt", () => {
    // Ohne den führenden Schrägstrich wäre "?tag=..." relativ zur aktuellen
    // Seite: der Zurück-Weg aus einer Klasse führte dann nirgendwo hin
    // (#2294).
    expect(schoolPath("/school?tag=2026-10-26")).toBe("/?tag=2026-10-26");
  });

  it("behält den Query einer Unterseite", () => {
    expect(schoolPath("/school/klasse/2a?tag=2026-10-26")).toBe(
      "/klasse/2a?tag=2026-10-26",
    );
  });
});
