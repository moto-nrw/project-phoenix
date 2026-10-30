import { describe, expect, it } from "vitest";

import {
  PLANNING_SUB_PAGES,
  getActivePlanningSubPageHref,
  isPlanningPageHref,
} from "./planning-navigation";
import {
  COMMUNICATION_SUB_PAGES,
  DATABASE_SUB_PAGES,
  ENROLLMENT_SUB_PAGES,
  PARENT_SUB_PAGES,
  STAFF_FLAT_PAGES,
} from "./section-navigation";

describe("planning navigation", () => {
  it.each([
    ["/betreuungsplan", "/betreuungsplan"],
    ["/betreuungsplan/42", "/betreuungsplan"],
    ["/dienstplan", "/dienstplan"],
    ["/vertretung", "/vertretung"],
    ["/lists", "/lists"],
    ["/payroll", "/payroll"],
    ["/calendar-periods", "/calendar-periods"],
  ] as const)("maps %s to %s", (pathname, expected) => {
    expect(getActivePlanningSubPageHref(pathname)).toBe(expected);
  });

  it("uses path-segment boundaries", () => {
    expect(getActivePlanningSubPageHref("/vertretungsplaner")).toBeNull();
    expect(getActivePlanningSubPageHref("/dienstplan-archiv")).toBeNull();
  });

  it.each(["/planung", "/timetables", "/staff/dienstplan", "/vertretungsplan"])(
    "leaves the server-only redirect %s out of client navigation",
    (pathname) => {
      expect(getActivePlanningSubPageHref(pathname)).toBeNull();
    },
  );

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

describe("navigation labels follow the module catalog (#4778)", () => {
  const labels = [
    ...PLANNING_SUB_PAGES,
    ...DATABASE_SUB_PAGES,
    ...COMMUNICATION_SUB_PAGES,
    ...PARENT_SUB_PAGES,
    ...ENROLLMENT_SUB_PAGES,
    ...Object.values(STAFF_FLAT_PAGES),
  ].map((page) => page.label);

  it("names exactly one navigation entry with the word Vertretung", () => {
    expect(labels.filter((label) => label.includes("Vertretung"))).toEqual([
      "Vertretungsplan",
    ]);
  });

  it("names the daily handover page Übergaben", () => {
    expect(STAFF_FLAT_PAGES.substitutions.label).toBe("Übergaben");
  });
});
