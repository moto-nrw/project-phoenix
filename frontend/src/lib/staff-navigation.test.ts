import { describe, expect, it } from "vitest";
import { STAFF_FLAT_PAGES } from "~/lib/section-navigation";
import {
  getStaffNavDefaultOpenGroups,
  getStaffNavGroupForHref,
  getStaffNavGroupForPathname,
  STAFF_NAV_BOTTOM,
  STAFF_NAV_GROUPS,
  STAFF_NAV_TOP,
} from "~/lib/staff-navigation";

describe("staff navigation tree", () => {
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

  it("keeps the one start page above and the pinned pages below the groups", () => {
    // Seit #2180 gibt es genau eine Startseite für alle Rollen; der Tagesplan
    // steht im Tagesbetrieb.
    expect(
      STAFF_NAV_TOP.map((entry) => entry.kind === "page" && entry.href),
    ).toEqual(["/home"]);
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
  });

  it("opens the group of the person's own work on a first visit (#6670)", () => {
    expect(getStaffNavDefaultOpenGroups(true)).toEqual([
      "tagesbetrieb",
      "planung",
    ]);
    expect(getStaffNavDefaultOpenGroups(false)).toEqual([
      "tagesbetrieb",
      "team",
    ]);
  });

  it("puts the own pages first in Team and the shared files last (#6670)", () => {
    const team = STAFF_NAV_GROUPS.find((group) => group.key === "team");
    const hrefs = (team?.entries ?? []).map((entry) =>
      entry.kind === "page" ? entry.href : entry.section,
    );
    expect(hrefs.slice(0, 3)).toEqual([
      STAFF_FLAT_PAGES.ownShiftPlan.href,
      STAFF_FLAT_PAGES.calendar.href,
      STAFF_FLAT_PAGES.timeTracking.href,
    ]);
    expect(hrefs.at(-1)).toBe(STAFF_FLAT_PAGES.dateien.href);
  });

  it("puts Alle Kinder directly below Meine Gruppen in the Tagesbetrieb (#4348)", () => {
    const tagesbetrieb = STAFF_NAV_GROUPS.find(
      (group) => group.key === "tagesbetrieb",
    );
    const entries = (tagesbetrieb?.entries ?? []).map((entry) =>
      entry.kind === "page" ? entry.href : entry.section,
    );
    expect(entries.slice(0, 4)).toEqual([
      STAFF_FLAT_PAGES.tagesplan.href,
      "groups",
      STAFF_FLAT_PAGES.studentSearch.href,
      "supervisions",
    ]);
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
      // #6670: Dateien zu Team, Ferienwochen zu Eltern, Abrechnung aus der
      // Planung in die Verwaltung.
      expect(getStaffNavGroupForHref("/dateien")).toBe("team");
      expect(getStaffNavGroupForHref("/ferienwochen")).toBe("eltern");
      expect(getStaffNavGroupForHref("/payroll")).toBe("verwaltung");
    });

    it("returns null for the start and pinned pages", () => {
      expect(getStaffNavGroupForHref("/home")).toBeNull();
      // Der Tagesplan gehört seit #2180 in den Tagesbetrieb.
      expect(getStaffNavGroupForHref("/tagesplan")).toBe("tagesbetrieb");
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

    it("counts the canonical planning paths", () => {
      expect(getStaffNavGroupForPathname("/dienstplan")).toBe("planung");
      expect(getStaffNavGroupForPathname("/betreuungsplan")).toBe("planung");
      expect(getStaffNavGroupForPathname("/vertretung")).toBe("planung");
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
