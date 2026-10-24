import { describe, expect, it } from "vitest";
import {
  resolveStudentReferrer,
  studentHistoryBack,
  studentReferrerBackLabel,
} from "./student-referrer";

describe("student referrer", () => {
  it("rejects an external return path", () => {
    expect(resolveStudentReferrer("//attacker.example")).toBe(
      "/students/search",
    );
  });

  // Der Rückweg zeigt seinen Text auf jeder Breite (#4364): er muss die
  // Sammlung nennen, aus der man kam, nicht immer die Kinderübersicht.
  it.each([
    ["/students/search?q=mia", "Zurück zur Kinderübersicht"],
    ["/database/students", "Zurück zu den Kinderdaten"],
    ["/rooms/3?from=%2Frooms", "Zurück zu den Räumen"],
    ["/messages", "Zurück zu Nachrichten"],
    ["/ogs-groups", "Zurück zur Gruppe"],
  ])("names %s as %s", (referrer, label) => {
    expect(studentReferrerBackLabel(referrer)).toBe(label);
  });

  it("returns to the history tab and keeps a query in `from` intact", () => {
    expect(studentHistoryBack("7", "/rooms?building=Nord&x=1", false)).toEqual({
      backHref: `/students/7?from=${encodeURIComponent("/rooms?building=Nord&x=1")}&tab=historie`,
      backLabel: "Zurück zum Kinderprofil",
    });
  });

  it("returns to the list when the sub-page failed", () => {
    expect(studentHistoryBack("7", "/rooms", true)).toEqual({
      backHref: "/rooms",
      backLabel: "Zurück zu den Räumen",
    });
  });
});
