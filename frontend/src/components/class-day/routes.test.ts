import { describe, expect, it } from "vitest";

import {
  classDayDateParam,
  classDayNameFromParam,
  classDayOverviewPath,
  classDayPath,
  isWeekendISO,
} from "./routes";

/** Adress-Segment aus einem Pfad, wie `params` es der Seite liefert. */
function segmentOf(path: string): string {
  return path.slice("/school/klasse/".length).split("?")[0]!;
}

describe("classDayDateParam", () => {
  it("übernimmt einen gültigen Tag aus der Adresse", () => {
    expect(classDayDateParam("2026-10-26", "2026-08-24")).toBe("2026-10-26");
  });

  it("fällt ohne Angabe auf heute zurück", () => {
    expect(classDayDateParam(null, "2026-08-24")).toBe("2026-08-24");
    expect(classDayDateParam("", "2026-08-24")).toBe("2026-08-24");
  });

  it("fällt bei unbrauchbarer Angabe auf heute zurück", () => {
    // Eine kaputte Adresse darf die Übergabe nicht blockieren.
    expect(classDayDateParam("26.10.2026", "2026-08-24")).toBe("2026-08-24");
    expect(classDayDateParam("2026-13-99", "2026-08-24")).toBe("2026-08-24");
  });
});

describe("classDayPath", () => {
  it("kodiert Klassennamen mit Leerzeichen", () => {
    // Klassennamen sind Freitext: "Klasse 2a" ist genauso gültig wie "2a".
    expect(classDayPath("Klasse 2a", "2026-10-26")).toBe(
      "/school/klasse/Klasse%202a?tag=2026-10-26",
    );
  });

  it("trägt den Tag mit in die Klasse", () => {
    expect(classDayPath("2a", "2026-10-26")).toBe(
      "/school/klasse/2a?tag=2026-10-26",
    );
  });

  it("führt mit demselben Tag zurück", () => {
    // Hin- und Rückweg müssen sich über den Tag einig sein.
    expect(classDayOverviewPath("2026-10-26")).toBe("/school?tag=2026-10-26");
  });
});

describe("isWeekendISO", () => {
  it("erkennt Samstag und Sonntag", () => {
    expect(isWeekendISO("2026-10-24")).toBe(true);
    expect(isWeekendISO("2026-10-25")).toBe(true);
  });

  it("lässt Wochentage durch", () => {
    expect(isWeekendISO("2026-10-26")).toBe(false);
    expect(isWeekendISO("2026-10-30")).toBe(false);
  });
});

describe("classDayNameFromParam", () => {
  it("gewinnt den Klassennamen aus dem Segment zurück", () => {
    // Next.js reicht `params` roh durch: das Segment kommt kodiert an.
    expect(classDayNameFromParam("Klasse%202a")).toBe("Klasse 2a");
  });

  it("schließt den Kreis mit classDayPath", () => {
    // Kodieren und Dekodieren gehören zusammen, auch für Namen, die selbst
    // wie eine Kodierung aussehen oder ein Prozentzeichen tragen.
    for (const name of [
      "2a",
      "Klasse 2a",
      "1%20a",
      "100%",
      "5 b/c",
      "Ü-Klasse",
    ]) {
      expect(
        classDayNameFromParam(segmentOf(classDayPath(name, "2026-10-26"))),
      ).toBe(name);
    }
  });
});
