import { describe, expect, it } from "vitest";

import type { Student } from "~/lib/api";
import { searchStudentDay } from "./search-student-card";

function student(overrides: Partial<Student>): Student {
  return {
    id: "1",
    name: "Mia Kaya",
    first_name: "Mia",
    second_name: "Kaya",
    arrival_time: "08:00",
    pickup_time: "15:00",
    ...overrides,
  } as Student;
}

describe("searchStudentDay (#3834)", () => {
  it("carries the planned times of a child that comes", () => {
    const day = searchStudentDay(student({}), true);

    expect(day.absence).toBeUndefined();
    expect(day.arrival.arrivalTime).toBe("08:00");
    expect(day.pickup.pickupTime).toBe("15:00");
  });

  it("names a known absence instead of times, like the card", () => {
    const day = searchStudentDay(
      student({ sick: true, pickup_notes: "Fieber" }),
      true,
    );

    expect(day.absence?.label).toBeTruthy();
    expect(day.absence?.wording).toBeUndefined();
    expect(day.absence?.note).toBe("Fieber");
  });

  it("says „Kommt nicht“ instead of „heute“ on another day", () => {
    const day = searchStudentDay(student({ sick: true }), false);

    expect(day.absence?.wording).toBe("Kommt nicht");
  });

  it("shows the times once the child has been picked up despite an absence", () => {
    const day = searchStudentDay(
      student({ sick: true, actual_pickup_time: "12:00" }),
      true,
    );

    expect(day.absence).toBeUndefined();
    expect(day.pickup.actualTime).toBe("12:00");
  });
});
