import { describe, expect, it } from "vitest";

import {
  getDayPlanningNotComingLabel,
  getStudentPresenceBadgePlanning,
} from "./day-planning-helper";

describe("getStudentPresenceBadgePlanning unplanned presence", () => {
  it("keeps a planned absence visible while the child is at home", () => {
    expect(
      getStudentPresenceBadgePlanning({
        current_location: "Zuhause",
        day_planning_status: "not_coming_today",
        day_planning_label: "Kommt heute nicht",
      }),
    ).toEqual({
      notArrivalToday: true,
      notArrivalReason: "Kommt heute nicht",
    });
  });

  it("marks actual attendance that contradicts the plan", () => {
    expect(
      getStudentPresenceBadgePlanning({
        current_location: "Anwesend - Gruppenraum",
        day_planning_status: "comes_today",
        day_planning_reason: "unplanned_attendance",
        day_planning_label: "ungeplant anwesend",
        actual_arrival_time: "08:30",
      }),
    ).toEqual({
      notArrivalToday: true,
      notArrivalReason: "ungeplant anwesend",
    });
  });

  it("restores the planned absence after checkout", () => {
    expect(
      getStudentPresenceBadgePlanning({
        current_location: "Zuhause",
        day_planning_status: "not_coming_today",
        day_planning_reason: "arrival_exception",
        day_planning_label: "Kommt heute nicht",
        actual_arrival_time: "08:30",
      }),
    ).toEqual({
      notArrivalToday: true,
      notArrivalReason: "Kommt heute nicht",
    });
  });

  it("restores the planned absence for composite home locations after checkout", () => {
    expect(
      getStudentPresenceBadgePlanning({
        current_location: "Zuhause - Abgeholt",
        day_planning_status: "not_coming_today",
        day_planning_reason: "arrival_exception",
        day_planning_label: "Kommt heute nicht",
        actual_arrival_time: "08:30",
      }),
    ).toEqual({
      notArrivalToday: true,
      notArrivalReason: "Kommt heute nicht",
    });
  });

  it("keeps an active transit check-in marked as unplanned", () => {
    expect(
      getStudentPresenceBadgePlanning({
        current_location: "Unterwegs",
        day_planning_status: "comes_today",
        day_planning_reason: "unplanned_attendance",
        day_planning_label: "ungeplant anwesend",
      }),
    ).toEqual({
      notArrivalToday: true,
      notArrivalReason: "ungeplant anwesend",
    });
  });

  it("does not mark a normally planned check-in as unplanned", () => {
    expect(
      getStudentPresenceBadgePlanning({
        current_location: "Anwesend - Gruppenraum",
        day_planning_status: "comes_today",
        day_planning_reason: "pickup_schedule",
        day_planning_label: "Abholplan heute",
        actual_arrival_time: "08:30",
      }),
    ).toEqual({
      notArrivalToday: false,
      notArrivalReason: null,
    });
  });
});

// #5710: Ein krank gemeldetes Kind, das das Personal zum Unterricht abgemeldet
// hat, war heute da. Die Karte sagt dann nicht „Kommt heute nicht“.
describe("getDayPlanningNotComingLabel after a checkout to class", () => {
  const sickPlan = {
    day_planning_status: "not_coming_today" as const,
    day_planning_label: "krank gemeldet",
  };

  it("drops the not-coming label for a child checked out to class", () => {
    expect(
      getDayPlanningNotComingLabel({
        ...sickPlan,
        current_location: "Im Unterricht",
      }),
    ).toBeNull();
  });

  it("keeps it before the first check-in and for another day", () => {
    expect(
      getDayPlanningNotComingLabel({ ...sickPlan, current_location: "Schule" }),
    ).toBe("krank gemeldet");
    expect(
      getDayPlanningNotComingLabel(
        { ...sickPlan, current_location: "Im Unterricht" },
        { ignoreCurrentAttendance: true },
      ),
    ).toBe("krank gemeldet");
  });
});
