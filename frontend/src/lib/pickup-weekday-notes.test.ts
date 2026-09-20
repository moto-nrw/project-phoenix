import { describe, expect, it } from "vitest";
import {
  getDayData,
  mapPickupNoteFormToBackend,
  mapPickupNoteResponse,
  type PickupNote,
} from "./pickup-schedule-helpers";

// #3369: a note that recurs on a weekday and needs no pickup time.

function note(overrides: Partial<PickupNote>): PickupNote {
  return {
    id: "1",
    studentId: "42",
    noteDate: "",
    content: "Notiz",
    createdBy: "1",
    createdAt: "2026-09-01T00:00:00Z",
    updatedAt: "2026-09-01T00:00:00Z",
    ...overrides,
  };
}

describe("recurring weekday notes", () => {
  it("maps a recurring note without a date", () => {
    const mapped = mapPickupNoteResponse({
      id: 7,
      student_id: 42,
      weekday: 3,
      content: "Mittwochs bei den Großeltern",
      created_by: 1,
      created_at: "2026-09-01T00:00:00Z",
      updated_at: "2026-09-01T00:00:00Z",
    });

    expect(mapped.weekday).toBe(3);
    expect(mapped.noteDate).toBe("");
  });

  it("sends either the weekday or the date, never both", () => {
    expect(mapPickupNoteFormToBackend({ weekday: 3, content: "x" })).toEqual({
      weekday: 3,
      content: "x",
    });
    expect(
      mapPickupNoteFormToBackend({ noteDate: "2026-09-09", content: "x" }),
    ).toEqual({ note_date: "2026-09-09", content: "x" });
  });

  it("puts the recurring note on its weekday without inventing a pickup time", () => {
    const notes = [
      note({ id: "1", weekday: 3, content: "Mittwochs bei den Großeltern" }),
      note({ id: "2", noteDate: "2026-09-09", content: "Nur heute" }),
    ];

    // 2026-09-09 is a Wednesday.
    const wednesday = getDayData(new Date(2026, 8, 9), [], [], false, notes);
    expect(wednesday.weekdayNote?.content).toBe("Mittwochs bei den Großeltern");
    expect(wednesday.notes.map((entry) => entry.id)).toEqual(["2"]);
    expect(wednesday.effectiveTime).toBeUndefined();

    const thursday = getDayData(new Date(2026, 8, 10), [], [], false, notes);
    expect(thursday.weekdayNote).toBeUndefined();
  });
});
