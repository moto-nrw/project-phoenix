import { describe, expect, it } from "vitest";
import {
  OGS_GROUP_SECTION_LABELS,
  ogsGroupSectionOf,
  parseOgsGroupSection,
} from "./ogs-group-sections";

describe("ogsGroupSectionOf", () => {
  it("puts a group without the personal flag under „Weitere Gruppen“", () => {
    expect(OGS_GROUP_SECTION_LABELS[ogsGroupSectionOf(false)]).toBe(
      "Weitere Gruppen",
    );
  });

  it("treats a missing flag as an own group, like the sidebar", () => {
    expect(ogsGroupSectionOf(true)).toBe("personal");
    expect(ogsGroupSectionOf(undefined)).toBe("personal");
  });
});

describe("parseOgsGroupSection", () => {
  it("accepts only the two known sections", () => {
    expect(parseOgsGroupSection("other")).toBe("other");
    expect(parseOgsGroupSection("personal")).toBe("personal");
    expect(parseOgsGroupSection("Weitere Gruppen")).toBeUndefined();
    expect(parseOgsGroupSection(null)).toBeUndefined();
  });
});
