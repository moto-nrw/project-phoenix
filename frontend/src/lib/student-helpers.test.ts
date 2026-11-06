import { describe, it, expect } from "vitest";
import { getSchoolYear } from "./student-helpers";

describe("getSchoolYear", () => {
  it("extracts the first number from class labels", () => {
    expect(getSchoolYear("Klasse 3a")).toBe("3");
    expect(getSchoolYear("2b")).toBe("2");
  });

  it("returns null when the class label has no number", () => {
    expect(getSchoolYear("unknown")).toBeNull();
  });
});
