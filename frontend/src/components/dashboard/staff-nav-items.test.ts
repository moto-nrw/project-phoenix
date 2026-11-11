import type { Session } from "next-auth";
import { describe, expect, it } from "vitest";

import {
  COMMUNICATION_SUB_PAGES,
  PARENT_SUB_PAGES,
  STAFF_FLAT_PAGES,
} from "~/lib/section-navigation";
import { PLANNING_SUB_PAGES } from "~/lib/planning-navigation";
import {
  STAFF_NAV_BOTTOM,
  STAFF_NAV_CONCEPTS,
  STAFF_NAV_GROUPS,
  STAFF_NAV_TOP,
} from "~/lib/staff-navigation";
import {
  STAFF_NAV_BINARY_HIDDEN_HREFS,
  STAFF_NAV_ITEMS,
  STAFF_NAV_ITEMS_BY_HREF,
  staffNavBorrowedActiveHref,
  staffNavItemPermitted,
  staffNavPathActive,
  staffNavSettingOn,
  type StaffNavItem,
} from "./staff-nav-items";

// Seitenleiste und mobile Leiste lesen ihre Seiten aus einer Liste (#6580).

const CATALOG_HREFS = new Set(
  [...PARENT_SUB_PAGES, ...COMMUNICATION_SUB_PAGES, ...PLANNING_SUB_PAGES].map(
    (page) => page.href,
  ),
);

const TREE_PAGE_HREFS = [
  ...STAFF_NAV_TOP,
  ...STAFF_NAV_GROUPS.flatMap((group) => group.entries),
  ...STAFF_NAV_BOTTOM,
].flatMap((entry) => (entry.kind === "page" ? [entry.href] : []));

function session(permissions: string[]): Session {
  return {
    user: { id: "1", permissions },
    expires: "2099-01-01",
  } as unknown as Session;
}

const STAFF = { isAdmin: false, isCaregiver: true, openCareGroupMode: false };

describe("staff navigation items (#6580)", () => {
  it("lists every page of the tree once, here or in a catalog with an image", () => {
    for (const href of TREE_PAGE_HREFS) {
      const inList = STAFF_NAV_ITEMS_BY_HREF.has(href);
      const inCatalog = CATALOG_HREFS.has(href);
      expect({ href, sources: Number(inList) + Number(inCatalog) }).toEqual({
        href,
        sources: 1,
      });
      if (inCatalog) expect(STAFF_NAV_CONCEPTS[href]).toBeDefined();
    }
  });

  it("has no entry outside the tree and no href twice", () => {
    const hrefs = STAFF_NAV_ITEMS.map((item) => item.href);
    expect(new Set(hrefs).size).toBe(hrefs.length);
    expect(hrefs.filter((href) => !TREE_PAGE_HREFS.includes(href))).toEqual([]);
  });

  it("hides the room and visit pages in binary mode", () => {
    expect([...STAFF_NAV_BINARY_HIDDEN_HREFS].sort()).toEqual(
      ["/activities", "/rooms", "/tagesplan"].sort(),
    );
  });

  describe("staffNavItemPermitted", () => {
    const item = (fields: Partial<StaffNavItem>): StaffNavItem => ({
      href: "/x",
      label: "X",
      concept: STAFF_NAV_ITEMS[0]!.concept,
      ...fields,
    });

    it("needs one of the listed permissions", () => {
      const rooms = item({ requiresPermission: ["rooms:read", "x:y"] });
      expect(
        staffNavItemPermitted(rooms, { ...STAFF, session: session([]) }),
      ).toBe(false);
      expect(
        staffNavItemPermitted(rooms, {
          ...STAFF,
          session: session(["rooms:read"]),
        }),
      ).toBe(true);
    });

    it("needs all permissions of requiresAllPermissions", () => {
      const statistics = item({
        requiresAllPermissions: ["config:read", "users:read"],
      });
      expect(
        staffNavItemPermitted(statistics, {
          ...STAFF,
          session: session(["config:read"]),
        }),
      ).toBe(false);
      expect(
        staffNavItemPermitted(statistics, {
          ...STAFF,
          session: session(["config:read", "users:read"]),
        }),
      ).toBe(true);
    });

    it("keeps admin-only pages from others, opens open-care pages by their permission", () => {
      const planning = item({
        requiresPermission: "groups:manage",
        openCarePermission: "groups:update",
      });
      const viewer = { ...STAFF, session: session(["groups:update"]) };
      expect(staffNavItemPermitted(planning, viewer)).toBe(false);
      expect(
        staffNavItemPermitted(planning, { ...viewer, openCareGroupMode: true }),
      ).toBe(true);
      expect(
        staffNavItemPermitted(item({ requiresAdmin: true }), {
          ...viewer,
          openCareGroupMode: true,
        }),
      ).toBe(false);
    });

    it("lets admins pass but hides caregiver entries from them", () => {
      const admin = {
        isAdmin: true,
        isCaregiver: false,
        openCareGroupMode: false,
        session: session([]),
      };
      expect(
        staffNavItemPermitted(item({ requiresPermission: "x:y" }), admin),
      ).toBe(true);
      expect(
        staffNavItemPermitted(
          item({ requiresPermission: "x:y", hideForAdmin: true }),
          admin,
        ),
      ).toBe(false);
      expect(
        staffNavItemPermitted(
          item({ requiresPermission: "x:y", hideForAdmin: true }),
          { ...admin, isCaregiver: true },
        ),
      ).toBe(true);
    });
  });

  it("drops a page whose school setting is off", () => {
    const dayLog = STAFF_NAV_ITEMS_BY_HREF.get(STAFF_FLAT_PAGES.dayLog.href)!;
    const all = { timetable: true, display: true, attendanceLog: true };
    expect(staffNavSettingOn(dayLog, all)).toBe(true);
    expect(staffNavSettingOn(dayLog, { ...all, attendanceLog: false })).toBe(
      false,
    );
    expect(staffNavSettingOn({}, { ...all, attendanceLog: false })).toBe(true);
  });

  it("does not let /calendar light up on /calendar-periods", () => {
    expect(staffNavPathActive("/calendar", "/calendar")).toBe(true);
    expect(staffNavPathActive("/calendar", "/calendar/3")).toBe(true);
    expect(staffNavPathActive("/calendar", "/calendar-periods")).toBe(false);
    expect(staffNavPathActive("/rooms", "/rooms/5")).toBe(true);
  });

  it("names the entry that marks a borrowed page", () => {
    expect(staffNavBorrowedActiveHref("/active-supervisions", null, true)).toBe(
      "/rooms",
    );
    expect(
      staffNavBorrowedActiveHref("/rooms/unterwegs", "/students/search", false),
    ).toBe("/students/search");
    expect(
      staffNavBorrowedActiveHref("/rooms/unterwegs", "/rooms", false),
    ).toBeNull();
  });
});
