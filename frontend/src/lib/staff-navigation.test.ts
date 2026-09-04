import { describe, expect, it } from "vitest";

import { PLANNING_SUB_PAGES } from "~/lib/planning-navigation";
import {
  COMMUNICATION_SUB_PAGES,
  PARENT_SUB_PAGES,
  STAFF_FLAT_PAGES,
} from "~/lib/section-navigation";
import {
  getStaffNavGroupForHref,
  getStaffNavGroupForPathname,
  listStaffNavPageHrefs,
  STAFF_NAV_BOTTOM,
  STAFF_NAV_DEFAULT_OPEN_GROUPS,
  STAFF_NAV_GROUPS,
  STAFF_NAV_TOP,
} from "~/lib/staff-navigation";

/**
 * Der Baum der Seitenleiste (#2826) ist die einzige Quelle für Gruppe und
 * Reihenfolge. Diese Tests fangen die zwei stillen Fehler: eine Katalogseite,
 * die nirgends steht (und damit aus Seitenleiste UND Mehr-Menü verschwindet),
 * und eine, die doppelt steht.
 */
describe("staff navigation tree", () => {
  const catalogHrefs = [
    ...Object.values(STAFF_FLAT_PAGES).map((page) => page.href),
    ...PARENT_SUB_PAGES.map((page) => page.href),
    ...COMMUNICATION_SUB_PAGES.map((page) => page.href),
    ...PLANNING_SUB_PAGES.map((page) => page.href),
  ];

  it("places every catalog page exactly once", () => {
    const placed = listStaffNavPageHrefs();
    for (const href of catalogHrefs) {
      expect(
        placed.filter((entry) => entry === href),
        href,
      ).toHaveLength(1);
    }
  });

  it("lists no page the catalogs do not know", () => {
    const known = new Set(catalogHrefs);
    for (const href of listStaffNavPageHrefs()) {
      expect(known.has(href), href).toBe(true);
    }
  });

  it("puts each accordion section in exactly one group", () => {
    const sections = STAFF_NAV_GROUPS.flatMap((group) =>
      group.entries.flatMap((entry) =>
        entry.kind === "section" ? [entry.section] : [],
      ),
    );
    expect([...sections].sort()).toEqual(
      ["database", "enrollments", "groups", "supervisions"].sort(),
    );
    expect(new Set(sections).size).toBe(sections.length);
  });

  it("keeps the role start pages above and the pinned pages below the groups", () => {
    expect(
      STAFF_NAV_TOP.map((entry) => entry.kind === "page" && entry.href),
    ).toEqual(["/dashboard", "/tagesplan"]);
    expect(
      STAFF_NAV_BOTTOM.map((entry) => entry.kind === "page" && entry.href),
    ).toEqual(["/emergency", "/help", "/settings"]);
  });

  it("orders the groups by how often the OGS day needs them", () => {
    expect(STAFF_NAV_GROUPS.map((group) => group.key)).toEqual([
      "tagesbetrieb",
      "eltern",
      "team",
      "planung",
      "verwaltung",
    ]);
    expect(STAFF_NAV_DEFAULT_OPEN_GROUPS).toEqual(["tagesbetrieb"]);
  });

  it("names no two groups with a shared word stem", () => {
    const labels = STAFF_NAV_GROUPS.map((group) => group.label.toLowerCase());
    for (const label of labels) {
      const stem = label.slice(0, 4);
      expect(labels.filter((other) => other.startsWith(stem))).toHaveLength(1);
    }
  });

  it("gives every group its own icon", () => {
    const icons = STAFF_NAV_GROUPS.map((group) => group.icon);
    expect(new Set(icons).size).toBe(icons.length);
  });

  describe("getStaffNavGroupForHref", () => {
    it("resolves catalog pages to their group", () => {
      expect(getStaffNavGroupForHref("/students/search")).toBe("tagesbetrieb");
      expect(getStaffNavGroupForHref("/anfragen")).toBe("tagesbetrieb");
      expect(getStaffNavGroupForHref("/messages")).toBe("eltern");
      expect(getStaffNavGroupForHref("/time-tracking")).toBe("team");
      expect(getStaffNavGroupForHref("/team-chat")).toBe("team");
      expect(getStaffNavGroupForHref("/dienstplan")).toBe("planung");
      expect(getStaffNavGroupForHref("/dateien")).toBe("verwaltung");
    });

    it("returns null for the start and pinned pages", () => {
      expect(getStaffNavGroupForHref("/dashboard")).toBeNull();
      expect(getStaffNavGroupForHref("/tagesplan")).toBeNull();
      expect(getStaffNavGroupForHref("/settings")).toBeNull();
    });
  });

  describe("getStaffNavGroupForPathname", () => {
    it("resolves sub-paths and accordion sections", () => {
      expect(getStaffNavGroupForPathname("/ogs-groups")).toBe("tagesbetrieb");
      expect(getStaffNavGroupForPathname("/active-supervisions")).toBe(
        "tagesbetrieb",
      );
      expect(getStaffNavGroupForPathname("/database/students/import")).toBe(
        "verwaltung",
      );
      expect(getStaffNavGroupForPathname("/enrollment-phases")).toBe("eltern");
      expect(getStaffNavGroupForPathname("/admin/enrollments/phases/3")).toBe(
        "eltern",
      );
      expect(getStaffNavGroupForPathname("/messages/42")).toBe("eltern");
    });

    it("counts the planning legacy paths", () => {
      expect(getStaffNavGroupForPathname("/staff/dienstplan")).toBe("planung");
      expect(getStaffNavGroupForPathname("/timetables")).toBe("planung");
      expect(getStaffNavGroupForPathname("/vertretungsplan")).toBe("planung");
    });

    it("does not let /calendar swallow /calendar-periods", () => {
      expect(getStaffNavGroupForPathname("/calendar")).toBe("team");
      expect(getStaffNavGroupForPathname("/calendar-periods")).toBe("planung");
    });

    it("keeps the student detail page in the group it was opened from", () => {
      expect(getStaffNavGroupForPathname("/students/7", "/ogs-groups")).toBe(
        "tagesbetrieb",
      );
      expect(getStaffNavGroupForPathname("/students/7", "/day-log")).toBe(
        "verwaltung",
      );
      expect(getStaffNavGroupForPathname("/students/7")).toBe("tagesbetrieb");
    });

    it("returns null for pages outside the groups", () => {
      expect(getStaffNavGroupForPathname("/dashboard")).toBeNull();
      expect(getStaffNavGroupForPathname("/profile")).toBeNull();
      expect(getStaffNavGroupForPathname("/settings")).toBeNull();
    });
  });
});
