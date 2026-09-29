import { describe, expect, it } from "vitest";

import { referenceInstant } from "./reference-time";

// Alle Zeiten sind UTC-Instants; 10:15 Berlin ist im Sommer 08:15Z (CEST) und
// im Winter 09:15Z (CET).
describe("referenceInstant", () => {
  it("nimmt den heutigen Werktag um 10:15, wenn 10:15 schon vorbei ist", () => {
    // Mittwoch 2026-09-09, 12:00 Berlin
    const now = new Date("2026-09-09T10:00:00Z");
    expect(referenceInstant(now).toISOString()).toBe(
      "2026-09-09T08:15:00.000Z",
    );
  });

  it("bleibt am heutigen Tag, auch wenn 10:15 noch bevorsteht", () => {
    // Mittwoch 08:00 Berlin: der Server rendert diesen Tag, die Uhr muss ihn
    // auch zeigen.
    const now = new Date("2026-09-09T06:00:00Z");
    expect(referenceInstant(now).toISOString()).toBe(
      "2026-09-09T08:15:00.000Z",
    );
  });

  it("nimmt am Wochenende den Freitag davor", () => {
    const friday = "2026-09-11T08:15:00.000Z";
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
      "2026-12-02T09:15:00.000Z",
    );
  });

  it("nimmt den Berliner Tag, nicht den UTC-Tag", () => {
    // Donnerstag 00:30 Berlin ist noch Mittwoch 22:30Z.
    const now = new Date("2026-09-09T22:30:00Z");
    expect(referenceInstant(now).toISOString()).toBe(
      "2026-09-10T08:15:00.000Z",
    );
  });
});
