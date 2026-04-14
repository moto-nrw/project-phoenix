import { describe, it, expect } from "vitest";
import { combinePickupNotes, getPickupUrgency } from "./pickup-helpers";

describe("combinePickupNotes", () => {
  it("returns undefined when no notes and no day notes", () => {
    expect(combinePickupNotes(undefined, undefined)).toBeUndefined();
  });

  it("returns notes only when no day notes", () => {
    expect(combinePickupNotes("Arzttermin", undefined)).toBe("Arzttermin");
  });

  it("returns notes only when day notes array is empty", () => {
    expect(combinePickupNotes("Arzttermin", [])).toBe("Arzttermin");
  });

  it("returns day notes only when no notes", () => {
    expect(combinePickupNotes(undefined, [{ content: "Früher abholen" }])).toBe(
      "Früher abholen",
    );
  });

  it("combines notes and day notes with comma separator", () => {
    expect(
      combinePickupNotes("Arzttermin", [{ content: "Früher abholen" }]),
    ).toBe("Arzttermin, Früher abholen");
  });

  it("combines notes with multiple day notes", () => {
    expect(
      combinePickupNotes("Termin", [
        { content: "Früher abholen" },
        { content: "Oma holt ab" },
      ]),
    ).toBe("Termin, Früher abholen, Oma holt ab");
  });

  it("skips empty day note content", () => {
    expect(
      combinePickupNotes("Termin", [{ content: "" }, { content: "Wichtig" }]),
    ).toBe("Termin, Wichtig");
  });

  it("returns undefined when notes is empty and day notes are empty", () => {
    expect(combinePickupNotes("", [{ content: "" }])).toBeUndefined();
  });
});

describe("getPickupUrgency", () => {
  it("returns 'none' when pickupTimeStr is undefined", () => {
    expect(getPickupUrgency(undefined, new Date())).toBe("none");
  });

  it("returns 'overdue' when pickup time is in the past", () => {
    const now = new Date("2025-01-15T15:00:00");
    expect(getPickupUrgency("14:30", now)).toBe("overdue");
  });

  it("returns 'soon' when pickup time is within 30 minutes", () => {
    const now = new Date("2025-01-15T14:45:00");
    expect(getPickupUrgency("15:00", now)).toBe("soon");
  });

  it("returns 'soon' when pickup time is exactly now (0 minutes diff)", () => {
    const now = new Date("2025-01-15T15:00:00");
    expect(getPickupUrgency("15:00", now)).toBe("soon");
  });

  it("returns 'normal' when pickup time is more than 30 minutes away", () => {
    const now = new Date("2025-01-15T13:00:00");
    expect(getPickupUrgency("15:00", now)).toBe("normal");
  });

  it("returns 'soon' at exactly 30 minutes before pickup", () => {
    const now = new Date("2025-01-15T14:30:00");
    expect(getPickupUrgency("15:00", now)).toBe("soon");
  });

  it("returns 'normal' at 31 minutes before pickup", () => {
    const now = new Date("2025-01-15T14:29:00");
    expect(getPickupUrgency("15:00", now)).toBe("normal");
  });
});
