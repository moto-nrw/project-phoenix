import { describe, expect, it } from "vitest";
import { TEST_CLOCK_TODAY } from "~/test/clock";
import {
  buildStudentSearchFilters,
  type StudentSearchFilterInput,
} from "./student-search-resource";

const base: StudentSearchFilterInput = {
  search: "",
  groupIds: [],
  schoolClasses: [],
  years: [],
  roomId: "",
  dayStatus: "all",
  date: TEST_CLOCK_TODAY,
  isToday: true,
  bus: "all",
  photoConsent: "all",
  pickupStatus: "all",
  companions: false,
};

describe("shared student search HTTP projection", () => {
  it("keeps the unfiltered today request slim and omits inactive filters", () => {
    expect(buildStudentSearchFilters(base)).toStrictEqual({
      search: "",
      groupId: [],
      schoolClass: [],
      gradeLevel: [],
      roomId: undefined,
      dayStatus: undefined,
      date: undefined,
      bus: undefined,
      photoConsent: undefined,
      pickupStatus: undefined,
      pageSize: undefined,
      includePickupTimes: true,
      includeArrivalTimes: true,
      includeCompanions: false,
      view: "slim",
    });
  });
  it("preserves planning, multi-selection and room pagination semantics", () => {
    expect(
      buildStudentSearchFilters({
        ...base,
        search: "Anna",
        groupIds: ["7", "9"],
        schoolClasses: ["1a"],
        years: ["1"],
        roomId: "12",
        dayStatus: "comes_today",
        isToday: false,
        bus: "yes",
        photoConsent: "no",
        pickupStatus: "self",
        companions: true,
      }),
    ).toStrictEqual({
      search: "Anna",
      groupId: ["7", "9"],
      schoolClass: ["1a"],
      gradeLevel: ["1"],
      roomId: "12",
      dayStatus: "comes_today",
      date: base.date,
      bus: "yes",
      photoConsent: "no",
      pickupStatus: "self",
      pageSize: 1000,
      includePickupTimes: true,
      includeArrivalTimes: true,
      includeCompanions: true,
      view: "slim",
    });
  });
});
