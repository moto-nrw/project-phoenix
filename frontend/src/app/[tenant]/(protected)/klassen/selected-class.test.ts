import { describe, expect, it } from "vitest";
import { reconcileSelectedClass } from "./selected-class";

describe("reconcileSelectedClass", () => {
  it("keeps a selection that is still assigned", () => {
    expect(reconcileSelectedClass("1b", ["1a", "1b"])).toBe("1b");
  });

  it("falls back to the first class when the selection was withdrawn", () => {
    // Admin entzieht "1b", die Fokus-Revalidierung liefert die neue Liste:
    // die Auswahl muss auf "1a" zurückfallen, statt eine Klasse ohne Report
    // und ohne Fehlerzustand anzuzeigen.
    expect(reconcileSelectedClass("1b", ["1a"])).toBe("1a");
  });

  it("falls back to the first class when nothing is selected yet", () => {
    expect(reconcileSelectedClass("", ["1a", "1b"])).toBe("1a");
  });

  it("returns empty while the class list is still loading", () => {
    expect(reconcileSelectedClass("1b", undefined)).toBe("");
  });

  it("returns empty when no classes are assigned", () => {
    expect(reconcileSelectedClass("1b", [])).toBe("");
  });
});
