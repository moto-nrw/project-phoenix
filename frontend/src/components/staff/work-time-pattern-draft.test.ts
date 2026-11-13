import { describe, expect, it } from "vitest";

import {
  draftFromPattern,
  patternEntries,
  rotationTotal,
  validatePatternDraft,
  weekTotal,
  withDecimalHours,
  withRotationLength,
  withStartTime,
} from "./work-time-pattern-draft";

describe("work-time pattern draft", () => {
  it("fills every weekday of every rotation week and shows minutes as decimal hours", () => {
    const draft = draftFromPattern(2, [
      { weekIndex: 0, dayOfWeek: 0, targetMinutes: 270, startTime: "08:00" },
      { weekIndex: 1, dayOfWeek: 4, targetMinutes: 60 },
    ]);
    expect(draft.entries).toHaveLength(10);
    expect(draft.hourInputs["0:0"]).toBe("4,5");
    expect(draft.hourInputs["0:1"]).toBe("");
    expect(weekTotal(draft, 0)).toBe(270);
    expect(rotationTotal(draft)).toBe(330);
  });

  it("parses typed hours and keeps an invalid entry visible but at zero", () => {
    let draft = draftFromPattern(1, []);
    draft = withDecimalHours(draft, 0, 2, "7.5");
    expect(weekTotal(draft, 0)).toBe(450);
    draft = withDecimalHours(draft, 0, 3, "13");
    expect(draft.hourInputs["0:3"]).toBe("13");
    expect(draft.invalidKeys.has("0:3")).toBe(true);
    expect(weekTotal(draft, 0)).toBe(450);
    expect(validatePatternDraft(draft)).toBe(
      "Bitte korrigieren Sie die markierten Stunden.",
    );
    draft = withDecimalHours(draft, 0, 3, "");
    expect(draft.invalidKeys.size).toBe(0);
  });

  it("drops the inputs and errors of rotation weeks that were removed", () => {
    let draft = draftFromPattern(2, [
      { weekIndex: 1, dayOfWeek: 0, targetMinutes: 120 },
    ]);
    draft = withDecimalHours(draft, 1, 1, "x");
    draft = withRotationLength(draft, 1);
    expect(draft.entries).toHaveLength(5);
    expect(draft.invalidKeys.size).toBe(0);
    expect(Object.keys(draft.hourInputs).some((k) => k.startsWith("1:"))).toBe(
      false,
    );
    expect(rotationTotal(draft)).toBe(0);
  });

  it("refuses an empty pattern and an empty rotation week", () => {
    expect(validatePatternDraft(draftFromPattern(1, []))).toBe(
      "Das Modell hat noch keine Stunden. Bitte tragen Sie mindestens einen Tag ein.",
    );
    const onlyWeekA = draftFromPattern(2, [
      { weekIndex: 0, dayOfWeek: 0, targetMinutes: 60 },
    ]);
    expect(validatePatternDraft(onlyWeekA)).toBe(
      "Woche B hat noch keine Stunden. Bitte tragen Sie mindestens einen Tag ein.",
    );
  });

  it("sends only days with hours, with their start time", () => {
    let draft = draftFromPattern(1, [
      { weekIndex: 0, dayOfWeek: 0, targetMinutes: 60 },
    ]);
    draft = withStartTime(draft, 0, 0, "07:30");
    draft = withStartTime(draft, 0, 1, "09:00");
    expect(validatePatternDraft(draft)).toBeNull();
    expect(patternEntries(draft)).toEqual([
      { weekIndex: 0, dayOfWeek: 0, targetMinutes: 60, startTime: "07:30" },
    ]);
  });
});
