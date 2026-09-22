import { describe, expect, it } from "vitest";
import {
  getHelpTopics,
  HELP_GROUP_LABELS,
  helpTopicMatchesRole,
  type HelpTopicGroup,
} from "~/components/help/help-content";
import type { SchoolSetupState } from "~/lib/school-setup-api";
import {
  applicableSteps,
  fillTopics,
  firstOpenStep,
  NEXT_HELP_GROUPS,
  setupProgress,
} from "./school-setup-steps";
import { CHILD_RECORD } from "./setup-tours";

function state(overrides: Partial<SchoolSetupState> = {}): SchoolSetupState {
  return {
    completed: false,
    dismissed: false,
    basics: {
      presenceMode: "detailed",
      groupMode: "fixed_groups",
      timetableEnabled: true,
      parentAppUsed: null,
    },
    steps: [
      { key: "basics", applies: true, done: true, skipped: false },
      { key: "team", applies: true, done: false, skipped: true },
      { key: "rooms", applies: false, done: false, skipped: false },
      { key: "groups", applies: true, done: false, skipped: false },
      { key: "students", applies: true, done: false, skipped: false },
      { key: "guardians", applies: false, done: false, skipped: false },
    ],
    ...overrides,
  };
}

describe("school setup steps", () => {
  it("only lists the steps that apply to the school", () => {
    expect(applicableSteps(state()).map((step) => step.key)).toEqual([
      "basics",
      "team",
      "groups",
      "students",
    ]);
  });

  it("starts at the first step that is neither done nor skipped", () => {
    expect(firstOpenStep(state())).toBe("groups");
  });

  it("counts done and skipped steps as finished", () => {
    expect(setupProgress(state())).toEqual({ finished: 2, total: 4 });
  });

  it("reaches the closing screen once nothing is open", () => {
    const finished = state({
      steps: [
        { key: "basics", applies: true, done: true, skipped: false },
        { key: "team", applies: true, done: false, skipped: true },
      ],
    });
    expect(firstOpenStep(finished)).toBeNull();
  });

  it("points to the guides for the remaining data of the steps that apply", () => {
    expect(fillTopics(state().steps).map((topic) => topic.title)).toEqual([
      "Das ganze Team einladen",
      "Alle Gruppen anlegen",
      "Alle Kinder übernehmen",
    ]);
  });

  it("points to help categories the lead sees, named as in the help", () => {
    const leadTopics = getHelpTopics("detailed", "fixed_groups", true).filter(
      (topic) => helpTopicMatchesRole(topic, "lead"),
    );
    for (const next of NEXT_HELP_GROUPS) {
      if (!("group" in next.link)) throw new Error("expected a help category");
      const group = next.link.group as HelpTopicGroup;
      // Derselbe Name wie auf der Seite der Hilfe, damit man sie wiederfindet.
      expect(next.title).toBe(HELP_GROUP_LABELS[group]);
      // Und die Kategorie hat für die Leitung auch Anleitungen.
      expect(leadTopics.some((topic) => topic.group === group)).toBe(true);
    }
    expect(NEXT_HELP_GROUPS.map((next) => next.title)).toContain("Anmeldungen");
  });
});

describe("CHILD_RECORD", () => {
  it("matches the child record, with or without a school prefix", () => {
    expect(CHILD_RECORD.test("/students/5")).toBe(true);
    expect(CHILD_RECORD.test("/ogs-nord/students/5")).toBe(true);
  });

  it("does not match the data management pages of children", () => {
    expect(CHILD_RECORD.test("/database/students")).toBe(false);
    expect(CHILD_RECORD.test("/database/students/import")).toBe(false);
    expect(CHILD_RECORD.test("/students/5/extra")).toBe(false);
  });
});
