import { describe, expect, it } from "vitest";
import type {
  SchoolDayReport,
  SchoolDayTotals,
} from "~/lib/school-class-day-api";
import {
  schoolDayChildrenByClass,
  type SchoolDayListKey,
  schoolDayListParam,
  schoolDayListPath,
  visibleSchoolDayLists,
} from "./school-day";

const zero: SchoolDayTotals = {
  students: 0,
  missing: 0,
  absent: 0,
  sick: 0,
  excused: 0,
  class_trip: 0,
  staying: 0,
  leaving: 0,
  list_entries: 0,
  changes: 0,
};

const report: SchoolDayReport = {
  date: "2026-09-09",
  weekday: "wed",
  school_day: true,
  enrollment_known: true,
  totals: { ...zero, students: 3, missing: 1, sick: 1, staying: 1, leaving: 1 },
  classes: [
    {
      school_class: "1a",
      totals: zero,
      children: [
        {
          student_id: "1",
          first_name: "Anna",
          last_name: "Arndt",
          after_school: "stays",
        },
        {
          student_id: "2",
          first_name: "Ben",
          last_name: "Busch",
          status: "sick",
          missing: true,
        },
      ],
    },
    { school_class: "2b", totals: zero, children: [] },
    {
      school_class: "3c",
      totals: zero,
      children: [
        {
          student_id: "3",
          first_name: "Carl",
          last_name: "Claus",
          after_school: "goes_home",
        },
      ],
    },
  ],
};

const list = (key: SchoolDayListKey) =>
  visibleSchoolDayLists(report).find((candidate) => candidate.key === key)!;

describe("school day lists", () => {
  it("reads only known lists from the address", () => {
    expect(schoolDayListParam("krank")).toBe("krank");
    expect(schoolDayListParam("gibt-es-nicht")).toBeNull();
    expect(schoolDayListParam(null)).toBeNull();
  });

  it("builds the address of a list for the same day", () => {
    expect(schoolDayListPath("2026-09-09", "gehen-heim")).toBe(
      "/school?tag=2026-09-09&liste=gehen-heim",
    );
    expect(schoolDayListPath("2026-09-09", null)).toBe(
      "/school?tag=2026-09-09",
    );
  });

  it("groups the children of a list by class and leaves out empty classes", () => {
    expect(schoolDayChildrenByClass(report, list("krank"))).toEqual([
      { schoolClass: "1a", children: [report.classes[0]!.children[1]] },
    ]);
    expect(
      schoolDayChildrenByClass(report, list("kinder")).map(
        (group) => group.schoolClass,
      ),
    ).toEqual(["1a", "3c"]);
    expect(
      schoolDayChildrenByClass(report, list("bleiben")).map(
        (group) => group.schoolClass,
      ),
    ).toEqual(["1a"]);
  });

  it("hides the stays and goes-home split without an enrollment phase", () => {
    const keys = (r: SchoolDayReport) =>
      visibleSchoolDayLists(r).map((list) => list.key);
    expect(keys(report)).toContain("bleiben");
    expect(keys({ ...report, enrollment_known: false })).toEqual([
      "kinder",
      "fehlen",
      "krank",
      "entschuldigt",
      "klassenfahrt",
    ]);
  });
});
