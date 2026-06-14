import { describe, expect, it } from "vitest";
import type { ArrivalSchedule } from "./student-arrival-api";
import { getDayData } from "./arrival-schedule-helpers";

describe("arrival schedule getDayData", () => {
  const schedules: ArrivalSchedule[] = [
    {
      id: 1,
      student_id: 100,
      weekday: 1,
      weekday_name: "Montag",
      expected_arrival: "08:30",
      notes: "Regulär",
      created_by: 5,
      created_at: "2026-05-01T10:00:00Z",
      updated_at: "2026-05-01T10:00:00Z",
    },
  ];

  it("treats class-trip status days as all-day absences", () => {
    const result = getDayData(
      new Date("2026-05-25T12:00:00"),
      schedules,
      [],
      [],
      false,
      false,
      "class_trip",
    );

    expect(result.showClassTrip).toBe(true);
    expect(result.showSick).toBe(false);
    expect(result.showExcused).toBe(false);
    expect(result.isAbsent).toBe(true);
    expect(result.effectiveTime).toBeUndefined();
    expect(result.effectiveReason).toBe("Klassenfahrt");
  });
});
