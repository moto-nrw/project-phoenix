import { describe, expect, it } from "vitest";
import { STAFF_ONBOARDING_STEP_KEYS } from "~/lib/staff-onboarding-api";
import {
  firstOpenStaffStep,
  STAFF_CHILD_RECORD,
  STAFF_ONBOARDING_STEP_CONTENT,
  STAFF_ONBOARDING_TOURS,
  staffOnboardingTours,
  staffOnboardingSteps,
} from "./staff-onboarding-steps";

const everything = {
  hasOwnGroup: true,
  webAttendance: true,
  canSeeCalendar: true,
  roomsTracked: true,
  canSearchChildren: true,
  canSupervise: true,
  canTrackWorkTime: true,
};

const noSchoolPreconditions = {
  hasOwnGroup: false,
  webAttendance: false,
  canSeeCalendar: false,
  roomsTracked: false,
  canSearchChildren: true,
  canSupervise: true,
  canTrackWorkTime: true,
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

  it("keeps finding a child and recording working time without school preconditions", () => {
    const steps = staffOnboardingSteps(
      { dismissed: false, schoolReady: true, doneSteps: [], skippedSteps: [] },
      noSchoolPreconditions,
    );

    expect(steps.map((step) => step.key)).toEqual(["students", "work_time"]);
  });

  // #4573: die Rolle „Extern“ hält nur schedules:read. Kein Schritt darf auf
  // eine Seite führen, die sie nicht öffnen darf.
  it("offers only the steps whose page the person may open", () => {
    const steps = staffOnboardingSteps(
      { dismissed: false, schoolReady: true, doneSteps: [], skippedSteps: [] },
      {
        ...everything,
        hasOwnGroup: false,
        canSeeCalendar: false,
        canSearchChildren: false,
        canTrackWorkTime: false,
      },
    );

    expect(steps.map((step) => step.key)).toEqual(["supervision"]);
  });

  it("has nothing open once every step is done or skipped", () => {
    const steps = staffOnboardingSteps(
      {
        dismissed: false,
        schoolReady: true,
        doneSteps: ["students"],
        skippedSteps: ["work_time"],
      },
      noSchoolPreconditions,
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

  // Kinder nur über Alle Kinder (#1321): ohne „Meine Gruppen“ und „Aktuelle
  // Aufsicht“ in der Navigation zeigen die Touren den Weg über die Suche.
  it("keeps the navigation tours while the school keeps the separate entries", () => {
    expect(staffOnboardingTours(false)).toBe(STAFF_ONBOARDING_TOURS);
  });

  it("leads to the group and the supervision through Alle Kinder", () => {
    const tours = staffOnboardingTours(true);
    const targets = (key: "groups" | "supervision") =>
      tours[key].stops.map((stop) => stop.target).join(" ");

    expect(tours.groups.path).toBe("/students/search");
    expect(targets("groups")).toContain("nav-/students/search");
    expect(targets("groups")).toContain('data-tab-value="groups"');
    expect(targets("groups")).not.toContain("nav-section-groups");

    expect(tours.supervision.path).toBe("/active-supervisions");
    expect(targets("supervision")).toContain("nav-/students/search");
    expect(targets("supervision")).toContain("child-search-menu");
    expect(targets("supervision")).toContain('href$="/active-supervisions"');
    expect(targets("supervision")).not.toContain("nav-section-supervisions");
    expect(tours.supervision.stops.at(-1)?.nav).toBeUndefined();

    // Die übrigen Touren bleiben, wie sie sind.
    expect(tours.students).toBe(STAFF_ONBOARDING_TOURS.students);
  });

  it("recognises the child record but not the search it comes from", () => {
    expect(STAFF_CHILD_RECORD.test("/students/42")).toBe(true);
    expect(STAFF_CHILD_RECORD.test("/ogs-am-see/students/42")).toBe(true);
    expect(STAFF_CHILD_RECORD.test("/students/search")).toBe(false);
    expect(STAFF_CHILD_RECORD.test("/ogs-am-see/students/search")).toBe(false);
    expect(STAFF_CHILD_RECORD.test("/database/students/42")).toBe(false);
  });
});
