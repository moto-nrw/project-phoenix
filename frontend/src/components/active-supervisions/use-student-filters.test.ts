import { act, renderHook } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { useStudentFilters } from "./use-student-filters";
import type { ActiveSupervisionStudent } from "./view-model";
import type { TimetableRosterRow } from "~/lib/timetable-operations-types";

function rosterRow(overrides: Partial<TimetableRosterRow>): TimetableRosterRow {
  return {
    studentId: "1",
    studentName: "Emil Fischer",
    schoolClass: "2a",
    groupName: "Sonnengruppe",
    planned: true,
    isUnplanned: false,
    currentlyPresent: false,
    visitId: null,
    status: "expected",
    substatus: null,
    note: null,
    checkedInAt: null,
    checkedOutAt: null,
    visitEntryTime: null,
    pickupTime: null,
    warnings: [],
    careDayStatus: "scheduled",
    parallelPresentIn: null,
    ...overrides,
  };
}

const NO_STUDENTS: readonly ActiveSupervisionStudent[] = [];

describe("useStudentFilters on a block list (#3889)", () => {
  it("leaves the block list alone while nothing is searched or filtered", () => {
    const { result } = renderHook(() => useStudentFilters(NO_STUDENTS));

    expect(result.current.rosterRowFilter).toBeNull();
  });

  it("searches block rows by name, ignoring case", () => {
    const { result } = renderHook(() => useStudentFilters(NO_STUDENTS));

    act(() => result.current.setSearchTerm("emil"));

    const filter = result.current.rosterRowFilter!;
    expect(filter(rosterRow({ studentName: "Emil Fischer" }))).toBe(true);
    expect(filter(rosterRow({ studentName: "Paul Becker" }))).toBe(false);
  });

  it("applies the year and group filters to block rows", () => {
    const { result } = renderHook(() => useStudentFilters(NO_STUDENTS));

    act(() => {
      result.current.setSelectedYear("2");
      result.current.setGroupFilter("Sonnengruppe");
    });

    const filter = result.current.rosterRowFilter!;
    expect(filter(rosterRow({}))).toBe(true);
    expect(filter(rosterRow({ schoolClass: "3b" }))).toBe(false);
    expect(filter(rosterRow({ groupName: "Waldgruppe" }))).toBe(false);
  });

  it("offers the groups of expected children in the group filter", () => {
    const rows = [
      rosterRow({ groupName: "Waldgruppe" }),
      rosterRow({ studentId: "2", groupName: "Sonnengruppe" }),
    ];
    const { result } = renderHook(() => useStudentFilters(NO_STUDENTS, rows));

    const group = result.current.filterConfigs.find((f) => f.id === "group");
    expect(group?.options?.map((option) => option.value)).toEqual([
      "all",
      "Sonnengruppe",
      "Waldgruppe",
    ]);
  });
});
