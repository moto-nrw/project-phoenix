import { describe, expect, it } from "vitest";
import {
  emptyParentConferenceDraft,
  parentConferenceFieldErrors,
  parentConferenceRequest,
  parentConferenceDraftOf,
  type ParentConference,
} from "./parent-conferences";

describe("parent conference form (#4801)", () => {
  const draft = {
    ...emptyParentConferenceDraft(["2a"]),
    date: "2026-11-12",
  };

  it("takes the only class and sends one break", () => {
    expect(draft.schoolClass).toBe("2a");
    expect(
      parentConferenceRequest({
        ...draft,
        breakStart: "15:00",
        breakEnd: "15:15",
        location: "  Raum 12 ",
      }),
    ).toEqual({
      school_class: "2a",
      date: "2026-11-12",
      start_time: "14:00",
      end_time: "17:00",
      slot_minutes: 15,
      breaks: [{ start_time: "15:00", end_time: "15:15" }],
      location: "Raum 12",
    });
  });

  it("preserves all breaks through an unrelated edit and edits only the first break", () => {
    const breaks = [
      { start_time: "14:30", end_time: "14:45" },
      { start_time: "15:00", end_time: "15:15" },
      { start_time: "16:00", end_time: "16:15" },
    ];
    const conference: ParentConference = {
      id: "5",
      ...parentConferenceRequest(draft),
      breaks,
      booked_count: 0,
      slots: [],
    };
    const editing = parentConferenceDraftOf(conference);
    expect(
      parentConferenceRequest({ ...editing, location: "Raum 13" }).breaks,
    ).toEqual(breaks);
    expect(
      parentConferenceRequest({ ...editing, breakStart: "14:15" }).breaks,
    ).toEqual([{ ...breaks[0], start_time: "14:15" }, ...breaks.slice(1)]);
    expect(
      parentConferenceRequest({ ...editing, breakStart: "", breakEnd: "" })
        .breaks,
    ).toEqual(breaks.slice(1));
  });

  it("names every wrong field", () => {
    expect(parentConferenceFieldErrors(draft, "2026-09-09")).toEqual({});
    expect(
      parentConferenceFieldErrors(
        { ...draft, date: "2026-09-01", endTime: "13:00" },
        "2026-09-09",
      ),
    ).toMatchObject({
      date: expect.any(String),
      end_time: expect.any(String),
    });
    expect(
      parentConferenceFieldErrors(
        { ...draft, breakStart: "13:00", breakEnd: "13:30" },
        "2026-09-09",
      ),
    ).toHaveProperty("breaks");
    expect(
      parentConferenceFieldErrors(
        { ...draft, breakStart: "15:00" },
        "2026-09-09",
      ),
    ).toHaveProperty("breaks");
  });
});
