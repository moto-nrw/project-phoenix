import { describe, expect, it } from "vitest";

import {
  PLANNING_SUB_PAGES,
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

  it("gives every planning page its own mobile entry", () => {
    // Früher hingen Tageslisten und Kalenderzeiträume als desktop-only Kinder
    // am Betreuungsplan-Eintrag. Sie waren dadurch mobil nicht erreichbar (es
    // führt kein Verweis vom Betreuungsplan dorthin), also markiert jetzt jede
    // Seite nur noch sich selbst und ihre eigenen Alt-Pfade.
    expect(getPlanningMobileActivePaths("/betreuungsplan")).toEqual([
      "/betreuungsplan",
      "/timetables",
    ]);
    expect(getPlanningMobileActivePaths("/lists")).toEqual(["/lists"]);
    expect(getPlanningMobileActivePaths("/calendar-periods")).toEqual([
      "/calendar-periods",
    ]);
  });

  it("keeps every planning page in the flattened mobile navigation", () => {
    expect(PLANNING_SUB_PAGES.filter((page) => !page.showInMobileNav)).toEqual(
      [],
    );
  });

  it("opens every planning page for a permission, not only for the admin role", () => {
    // Eine Leitungsrolle, die die Schule selbst anlegt (#3469), erreicht die
    // Planung über das Recht ihrer Route; ein Eintrag ohne Recht bliebe dem
    // Rollennamen `admin` vorbehalten.
    expect(
      PLANNING_SUB_PAGES.filter(
        (page) => page.nonAdminPermission === undefined,
      ),
    ).toEqual([]);
    expect(
      Object.fromEntries(
        PLANNING_SUB_PAGES.map((page) => [page.href, page.nonAdminPermission]),
      ),
    ).toEqual({
      "/betreuungsplan": "schedules:manage",
      "/dienstplan": "time_tracking:manage",
      "/vertretung": "schedules:manage",
      "/lists": "schedules:manage",
      "/calendar-periods": "schedules:manage",
      "/payroll": "config:manage",
    });
  });

  it("recognizes only canonical planning hrefs", () => {
    expect(isPlanningPageHref("/dienstplan")).toBe(true);
    expect(isPlanningPageHref("/staff/dienstplan")).toBe(false);
  });
});
