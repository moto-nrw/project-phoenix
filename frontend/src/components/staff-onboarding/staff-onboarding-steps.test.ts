import { describe, expect, it } from "vitest";
import { STAFF_ONBOARDING_STEP_KEYS } from "~/lib/staff-onboarding-api";
import {
  firstOpenStaffStep,
  STAFF_CHILD_RECORD,
  STAFF_ONBOARDING_STEP_CONTENT,
  STAFF_ONBOARDING_TOURS,
  staffOnboardingSteps,
} from "./staff-onboarding-steps";

const everything = {
  hasOwnGroup: true,
  webAttendance: true,
  canSeeCalendar: true,
  roomsTracked: true,
};

describe("staffOnboardingSteps", () => {
  it("keeps the order and marks done and skipped steps", () => {
    const steps = staffOnboardingSteps(
      {
        dismissed: false,
        schoolReady: true,
        doneSteps: ["students"],
        skippedSteps: ["groups"],
      },
      everything,
    );

    expect(steps.map((step) => step.key)).toEqual([
      ...STAFF_ONBOARDING_STEP_KEYS,
    ]);
    expect(steps[0]).toEqual({ key: "groups", done: false, skipped: true });
    expect(steps[1]).toEqual({ key: "students", done: true, skipped: false });
    expect(firstOpenStaffStep(steps)).toBe("attendance");
  });

  it("always keeps finding a child and recording working time", () => {
    const steps = staffOnboardingSteps(
      { dismissed: false, schoolReady: true, doneSteps: [], skippedSteps: [] },
      {
        hasOwnGroup: false,
        webAttendance: false,
        canSeeCalendar: false,
        roomsTracked: false,
      },
    );

    expect(steps.map((step) => step.key)).toEqual(["students", "work_time"]);
  });

  it("has nothing open once every step is done or skipped", () => {
    const steps = staffOnboardingSteps(
      {
        dismissed: false,
        schoolReady: true,
        doneSteps: ["students"],
        skippedSteps: ["work_time"],
      },
      {
        hasOwnGroup: false,
        webAttendance: false,
        canSeeCalendar: false,
        roomsTracked: false,
      },
    );

    expect(firstOpenStaffStep(steps)).toBeNull();
  });
});

describe("staff tours", () => {
  it("has content and a tour for every step, ending on its page", () => {
    for (const key of STAFF_ONBOARDING_STEP_KEYS) {
      expect(STAFF_ONBOARDING_STEP_CONTENT[key].title).not.toBe("");
      const tour = STAFF_ONBOARDING_TOURS[key];
      expect(tour.stops.length).toBeGreaterThan(1);
      expect(tour.stops.at(-1)?.nav).toBeUndefined();
    }
  });

  it("never asks the person to write data during a tour", () => {
    // Die Touren zeigen nur: Die letzte Station geht mit „Fertig“ weiter
    // oder mit einem Klick, der nur eine Ansicht öffnet.
    for (const tour of Object.values(STAFF_ONBOARDING_TOURS)) {
      for (const stop of tour.stops) {
        expect(stop.target).not.toMatch(/submit|Einstempeln/);
      }
    }
  });

  it("recognises the child record but not the search it comes from", () => {
    expect(STAFF_CHILD_RECORD.test("/students/42")).toBe(true);
    expect(STAFF_CHILD_RECORD.test("/ogs-am-see/students/42")).toBe(true);
    expect(STAFF_CHILD_RECORD.test("/students/search")).toBe(false);
    expect(STAFF_CHILD_RECORD.test("/ogs-am-see/students/search")).toBe(false);
    expect(STAFF_CHILD_RECORD.test("/database/students/42")).toBe(false);
  });
});
