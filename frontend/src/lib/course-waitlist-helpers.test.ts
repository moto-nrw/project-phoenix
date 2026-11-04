import { describe, expect, it } from "vitest";

import {
  mapCourseWaitlists,
  waitlistPlacesByRequest,
  waitlistsWithFreeSlot,
  type CourseWaitlistWire,
} from "./course-waitlist-helpers";

const theater: CourseWaitlistWire = {
  activity_group_id: "70",
  name: "Theater-AG",
  participant_limit: 8,
  participants: 8,
  free_slots: 0,
  slot_free_for_next: false,
  entries: [
    {
      request_id: "11",
      student_id: "101",
      student_name: "Lina Amini",
      rank: 1,
      submitted_at: "2026-09-01T10:00:00Z",
    },
    {
      request_id: "12",
      student_id: "102",
      student_name: "Mia Berg",
      rank: 2,
      submitted_at: "2026-09-02T10:00:00Z",
    },
  ],
};

const football: CourseWaitlistWire = {
  activity_group_id: "71",
  name: "Fußball-AG",
  participant_limit: 12,
  participants: 11,
  free_slots: 1,
  slot_free_for_next: true,
  entries: [
    {
      request_id: "12",
      student_id: "102",
      student_name: "Mia Berg",
      rank: 1,
      submitted_at: "2026-09-02T10:00:00Z",
    },
  ],
};

describe("course waitlist helpers", () => {
  it("maps the wire to camelCase and keeps a missing limit as null", () => {
    const [mapped] = mapCourseWaitlists([
      { ...theater, participant_limit: null, free_slots: null },
    ]);
    expect(mapped).toMatchObject({
      activityGroupId: "70",
      name: "Theater-AG",
      participantLimit: null,
      freeSlots: null,
    });
    expect(mapped?.entries[0]).toEqual({
      requestId: "11",
      studentId: "101",
      studentName: "Lina Amini",
      rank: 1,
      submittedAt: "2026-09-01T10:00:00Z",
    });
  });

  it("groups one request's places across AGs", () => {
    const places = waitlistPlacesByRequest(
      mapCourseWaitlists([theater, football]),
    );
    expect(
      places.get("12")?.map((place) => [place.waitlist.name, place.entry.rank]),
    ).toEqual([
      ["Theater-AG", 2],
      ["Fußball-AG", 1],
    ]);
    expect(places.get("11")).toHaveLength(1);
  });

  it("names only AGs with a free place and a waiting child", () => {
    const free = waitlistsWithFreeSlot(mapCourseWaitlists([theater, football]));
    expect(free.map((waitlist) => waitlist.name)).toEqual(["Fußball-AG"]);
  });
});
