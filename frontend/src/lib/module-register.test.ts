import { describe, expect, it } from "vitest";

import { HOME_BLOCKS } from "~/lib/home-blocks";
import {
  BASIS,
  HOME_BLOCK_MODULES,
  isScopeBooked,
  moduleOfPath,
  moduleVisibility,
  PAGE_MODULES,
  SECTION_MODULES,
  STAFF_TAB_MODULES,
} from "~/lib/module-register";
import { MODULE_KEYS } from "~/lib/modules";
import { PLANNING_SUB_PAGES } from "~/lib/planning-navigation";
import { DATABASE_SUB_PAGES } from "~/lib/section-navigation";
import {
  STAFF_NAV_BOTTOM,
  STAFF_NAV_GROUPS,
  STAFF_NAV_TOP,
} from "~/lib/staff-navigation";

const NAV_PAGE_HREFS = [
  ...STAFF_NAV_TOP,
  ...STAFF_NAV_GROUPS.flatMap((group) => group.entries),
  ...STAFF_NAV_BOTTOM,
].flatMap((entry) => (entry.kind === "page" ? [entry.href] : []));

const NAV_SECTION_KEYS = STAFF_NAV_GROUPS.flatMap((group) =>
  group.entries.flatMap((entry) =>
    entry.kind === "section" ? [entry.section] : [],
  ),
);

describe("module register (#4894)", () => {
  it("ordnet jede Seite der Navigation einem Modul oder der Basis zu", () => {
    const pages = [
      ...NAV_PAGE_HREFS,
      ...DATABASE_SUB_PAGES.map((page) => page.href),
      ...PLANNING_SUB_PAGES.flatMap((page) => [
        page.href,
        ...page.legacyPrefixes,
      ]),
    ];
    const missing = pages.filter((href) => PAGE_MODULES[href] === undefined);
    expect(missing).toEqual([]);
  });

  it("ordnet jeden Bereich der Seitenleiste zu", () => {
    for (const key of NAV_SECTION_KEYS) {
      expect(SECTION_MODULES[key]).toBeDefined();
    }
  });

  it("ordnet jeden Baustein der Startseite zu, und nur diese", () => {
    expect(Object.keys(HOME_BLOCK_MODULES).sort()).toEqual(
      HOME_BLOCKS.map((block) => block.key).sort(),
    );
  });

  it("kennt nur Module der Modulliste", () => {
    const scopes = [
      ...Object.values(PAGE_MODULES),
      ...Object.values(SECTION_MODULES),
      ...Object.values(HOME_BLOCK_MODULES),
      ...Object.values(STAFF_TAB_MODULES),
    ];
    const known = new Set<string>([BASIS, ...MODULE_KEYS]);
    expect(scopes.filter((scope) => !known.has(scope))).toEqual([]);
  });

  it("findet das Modul einer Adresse samt Unterseiten und alten Pfaden", () => {
    expect(moduleOfPath("/time-tracking")).toBe("zeiterfassung");
    expect(moduleOfPath("/admin/enrollments/42")).toBe("online_anmeldung");
    // Der längste Pfad gewinnt: /staff ist Basis, /staff/dienstplan nicht.
    expect(moduleOfPath("/staff/dienstplan")).toBe("dienstplan");
    expect(moduleOfPath("/staff/7")).toBeNull();
    expect(moduleOfPath("/database/devices")).toBe("nfc_anwesenheit");
    expect(moduleOfPath("/database/rooms")).toBeNull();
    // Kein blankes startsWith: /dienstplan-alt gehört nicht zu /dienstplan.
    expect(moduleOfPath("/dienstplan-alt")).toBeNull();
    expect(moduleOfPath("/unbekannt")).toBeNull();
  });

  it("zeigt ein nicht gebuchtes Modul der Leitung mit Schloss, sonst niemandem", () => {
    const booked = MODULE_KEYS.filter((key) => key !== "dienstplan");

    expect(moduleVisibility("dienstplan", booked, true)).toBe("locked");
    expect(moduleVisibility("dienstplan", booked, false)).toBe("hidden");
    expect(moduleVisibility("zeiterfassung", booked, false)).toBe("open");
    expect(moduleVisibility(BASIS, [], false)).toBe("open");
    // Unbekannte Adressen behandelt die Navigation wie Basis.
    expect(moduleVisibility(undefined, [], false)).toBe("open");
  });

  it("bucht ohne Liste nichts", () => {
    expect(isScopeBooked("mittagessen", [])).toBe(false);
    expect(isScopeBooked(BASIS, [])).toBe(true);
  });
});
