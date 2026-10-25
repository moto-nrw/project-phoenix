import { describe, expect, it } from "vitest";

import { referenceInstant } from "./reference-time";

// Alle Zeiten sind UTC-Instants; 13:30 Berlin ist im Sommer 11:30Z (CEST) und
// im Winter 12:30Z (CET).
describe("referenceInstant", () => {
  it("nimmt den heutigen Werktag um 13:30, wenn 13:30 schon vorbei ist", () => {
    // Mittwoch 2026-09-09, 15:00 Berlin
    const now = new Date("2026-09-09T13:00:00Z");
    expect(referenceInstant(now).toISOString()).toBe(
      "2026-09-09T11:30:00.000Z",
    );
  });

  it("bleibt am heutigen Tag, auch wenn 13:30 noch bevorsteht", () => {
    // Mittwoch 08:00 Berlin: der Server rendert diesen Tag, die Uhr muss ihn
    // auch zeigen.
    const now = new Date("2026-09-09T06:00:00Z");
    expect(referenceInstant(now).toISOString()).toBe(
      "2026-09-09T11:30:00.000Z",
    );
  });

  it("nimmt am Wochenende den Freitag davor", () => {
    const friday = "2026-09-11T11:30:00.000Z";
    expect(
      referenceInstant(new Date("2026-09-12T10:00:00Z")).toISOString(),
    ).toBe(friday);
    expect(
      referenceInstant(new Date("2026-09-13T10:00:00Z")).toISOString(),
    ).toBe(friday);
  });

  it("folgt der Winterzeit", () => {
    // Mittwoch 2026-12-02, 12:00 Berlin (CET)
    const now = new Date("2026-12-02T11:00:00Z");
    expect(referenceInstant(now).toISOString()).toBe(
      "2026-12-02T12:30:00.000Z",
    );
  });

  it("nimmt den Berliner Tag, nicht den UTC-Tag", () => {
    // Donnerstag 00:30 Berlin ist noch Mittwoch 22:30Z.
    const now = new Date("2026-09-09T22:30:00Z");
    expect(referenceInstant(now).toISOString()).toBe(
      "2026-09-10T11:30:00.000Z",
    );
  });
});
