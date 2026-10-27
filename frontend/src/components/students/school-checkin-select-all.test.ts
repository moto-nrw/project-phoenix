import { describe, expect, it } from "vitest";
import type { Student } from "~/lib/api";
import {
  isLeftOutOfSelectAll,
  leftOutLabel,
  planSelectAll,
} from "./school-checkin-select-all";

function student(id: string, fields: Partial<Student> = {}): Student {
  return {
    id,
    name: `Kind ${id}`,
    first_name: "Kind",
    second_name: id,
    school_class: "1a",
    current_location: "Schule",
    ...fields,
  } as Student;
}

describe("isLeftOutOfSelectAll (#4762)", () => {
  it.each([
    ["sick", { sick: true }],
    ["excused", { excused: true }],
    ["on a class trip", { class_trip: true }],
    [
      "not expected today",
      {
        day_planning_status: "not_coming_today" as const,
        day_planning_label: "Abgemeldet",
      },
    ],
    [
      "absent by an arrival exception",
      { arrival_is_exception: true, arrival_time: undefined },
    ],
  ])("leaves out a child who is %s", (_, fields) => {
    expect(isLeftOutOfSelectAll(student("1", fields))).toBe(true);
  });

  it("keeps an expected child", () => {
    expect(
      isLeftOutOfSelectAll(
        student("1", { day_planning_status: "comes_today" }),
      ),
    ).toBe(false);
  });

  it("keeps a child who is here although not expected", () => {
    expect(
      isLeftOutOfSelectAll(
        student("1", {
          current_location: "Anwesend - Raum 1",
          day_planning_status: "not_coming_today",
          arrival_is_exception: true,
        }),
      ),
    ).toBe(false);
  });
});

describe("planSelectAll", () => {
  it("returns the ids to mark in list order and counts the rest", () => {
    expect(
      planSelectAll([
        student("1"),
        student("2", { sick: true }),
        student("3"),
        student("4", { excused: true }),
      ]),
    ).toEqual({ ids: ["1", "3"], leftOutCount: 2 });
  });

  it("returns nothing for an empty list", () => {
    expect(planSelectAll([])).toEqual({ ids: [], leftOutCount: 0 });
  });
});

describe("leftOutLabel", () => {
  it("names how many children were left out and why", () => {
    expect(leftOutLabel(1)).toBe(
      "1 ausgelassen: krank, entschuldigt, auf Klassenfahrt oder „Kommt heute nicht“",
    );
  });

  it("is empty when nobody was left out", () => {
    expect(leftOutLabel(0)).toBeNull();
  });
});
