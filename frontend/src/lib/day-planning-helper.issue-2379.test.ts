import { describe, expect, it } from "vitest";

import { getStudentPresenceBadgePlanning } from "./day-planning-helper";

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
