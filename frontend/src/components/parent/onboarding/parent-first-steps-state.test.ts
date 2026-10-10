import { beforeEach, describe, expect, it } from "vitest";
import {
  completeParentFirstStep,
  parentFirstStepsStorageKey,
  readParentFirstStepsState,
  writeParentFirstStepsState,
} from "./parent-first-steps-state";

describe("parent first steps state", () => {
  beforeEach(() => localStorage.clear());

  it("keeps progress separate for each parent account", () => {
    writeParentFirstStepsState("12", {
      completed: ["start"],
      dismissed: false,
      finished: false,
    });

    expect(readParentFirstStepsState("12").completed).toEqual(["start"]);
    expect(readParentFirstStepsState("13").completed).toEqual([]);
    expect(parentFirstStepsStorageKey("12")).toContain(":12");
  });

  it("adds a completed tour only once", () => {
    const initial = readParentFirstStepsState("12");
    const completed = completeParentFirstStep(initial, "childData");
    expect(completeParentFirstStep(completed, "childData").completed).toEqual([
      "childData",
    ]);
  });

  it("ignores unknown and broken stored values", () => {
    localStorage.setItem(
      parentFirstStepsStorageKey("12"),
      JSON.stringify({
        completed: ["start", "unknown"],
        dismissed: "yes",
        finished: 1,
      }),
    );
    expect(readParentFirstStepsState("12")).toEqual({
      completed: ["start"],
      dismissed: false,
      finished: false,
    });

    localStorage.setItem(parentFirstStepsStorageKey("12"), "{");
    expect(readParentFirstStepsState("12").completed).toEqual([]);
  });
});
