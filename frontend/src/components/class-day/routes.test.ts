import { describe, expect, it } from "vitest";

import {
  classDayDateParam,
  classDayOverviewPath,
  classDayPath,
  isWeekendISO,
} from "./routes";

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
