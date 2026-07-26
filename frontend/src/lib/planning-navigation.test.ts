import { describe, expect, it } from "vitest";

import {
  getActivePlanningSubPageHref,
  getPlanningMobileActivePaths,
  isPlanningPageHref,
  isPlanningPath,
} from "./planning-navigation";

describe("planning navigation", () => {
  it.each([
    ["/betreuungsplan", "/betreuungsplan"],
    ["/betreuungsplan/42", "/betreuungsplan"],
    ["/timetables", "/betreuungsplan"],
    ["/dienstplan", "/dienstplan"],
    ["/staff/dienstplan", "/dienstplan"],
    ["/vertretung", "/vertretung"],
    ["/vertretungsplan", "/vertretung"],
    ["/calendar-periods", "/calendar-periods"],
  ] as const)("maps %s to %s", (pathname, expected) => {
    expect(getActivePlanningSubPageHref(pathname)).toBe(expected);
    expect(isPlanningPath(pathname)).toBe(true);
  });

  it("uses path-segment boundaries", () => {
    expect(getActivePlanningSubPageHref("/vertretungsplaner")).toBeNull();
    expect(isPlanningPath("/dienstplan-archiv")).toBe(false);
  });

  it("groups desktop-only calendar periods under Betreuungsplan on mobile", () => {
    expect(getPlanningMobileActivePaths("/betreuungsplan")).toEqual([
      "/betreuungsplan",
      "/timetables",
      "/lists",
      "/calendar-periods",
    ]);
  });

  it("recognizes only canonical planning hrefs", () => {
    expect(isPlanningPageHref("/dienstplan")).toBe(true);
    expect(isPlanningPageHref("/staff/dienstplan")).toBe(false);
  });
});
