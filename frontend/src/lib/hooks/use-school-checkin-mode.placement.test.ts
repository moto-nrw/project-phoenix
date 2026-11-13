import { describe, expect, it } from "vitest";

import { placementMessage, placementOutcome } from "./use-school-checkin-mode";

// A check-in into a room in the bulk mode (#6953) answers in the shape of the
// school check-in batch, so retry and failure handling stay one path.

describe("placementOutcome", () => {
  it("counts placed, kept and refused children from the server's answer", () => {
    const outcome = placementOutcome(["1", "2", "3", "9"], {
      moved: [1, 2],
      unchanged: [9],
      skipped: [{ student_id: 3, reason: "not_found" }],
      checked_in: [1],
    });
    expect(outcome.succeeded).toBe(3);
    expect(outcome.failed).toBe(1);
    expect(outcome.results).toEqual([
      { studentId: "1", ok: true, changed: true, location: "Anwesend" },
      { studentId: "2", ok: true, changed: false, location: "Anwesend" },
      { studentId: "3", ok: false, changed: false, error: "not_found" },
      { studentId: "9", ok: true, changed: false, location: "Anwesend" },
    ]);
  });
});

describe("placementMessage", () => {
  it("names the room and the children who stay where they are", () => {
    expect(placementMessage(1, 0, "Mensa")).toBe("1 Kind in Mensa angemeldet.");
    expect(placementMessage(5, 2, "Mensa")).toBe(
      "5 Kinder in Mensa angemeldet. 2 waren schon da und bleiben, wo sie sind.",
    );
    expect(placementMessage(1, 1, "Mensa")).toBe(
      "1 Kind in Mensa angemeldet. 1 war schon da und bleibt, wo es ist.",
    );
  });

  it("says when nobody needed a check-in", () => {
    expect(placementMessage(0, 3, "Mensa")).toBe(
      "Alle ausgewählten Kinder sind schon angemeldet.",
    );
  });
});
