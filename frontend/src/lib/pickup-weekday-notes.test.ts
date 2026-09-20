import { describe, expect, it } from "vitest";
import {
  getDayData,
  mapPickupNoteFormToBackend,
  mapPickupNoteResponse,
  planWeekdayNoteSync,
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

  it("plans only the writes that change something and leaves dated notes alone", () => {
    const stored = [
      note({ id: "1", weekday: 1, content: "bleibt" }),
      note({ id: "2", weekday: 2, content: "alt" }),
      note({ id: "3", weekday: 3, content: "fällt weg" }),
      note({ id: "4", noteDate: "2026-09-09", content: "datiert" }),
    ];

    expect(
      planWeekdayNoteSync(stored, [
        { weekday: 1, content: "bleibt" },
        { weekday: 2, content: "neu" },
        { weekday: 5, content: "kommt dazu" },
      ]),
    ).toEqual({
      create: [{ weekday: 5, content: "kommt dazu" }],
      update: [{ id: "2", entry: { weekday: 2, content: "neu" } }],
      remove: ["3"],
    });
  });
});
