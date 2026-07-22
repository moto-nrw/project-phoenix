export type PlanningPageHref =
  | "/betreuungsplan"
  | "/dienstplan"
  | "/vertretung"
  | "/lists"
  | "/calendar-periods";

export interface PlanningSubPage {
  readonly href: PlanningPageHref;
  readonly label: string;
  readonly legacyPrefixes: readonly string[];
  readonly showInMobileNav: boolean;
  readonly mobileParentHref?: PlanningPageHref;
}

/**
 * Single source of truth for planning navigation and its legacy redirects.
 * Kalenderzeiträume stays desktop-only and belongs to Betreuungsplan in the
 * flattened mobile navigation.
 */
export const PLANNING_SUB_PAGES: readonly PlanningSubPage[] = [
  {
    href: "/betreuungsplan",
    label: "Betreuungsplan",
    legacyPrefixes: ["/timetables"],
    showInMobileNav: true,
  },
  {
    href: "/dienstplan",
    label: "Dienstplan",
    legacyPrefixes: ["/staff/dienstplan"],
    showInMobileNav: true,
  },
  {
    href: "/vertretung",
    label: "Vertretung",
    legacyPrefixes: ["/vertretungsplan"],
    showInMobileNav: true,
  },
  {
    // Tageslisten (#1565): druckbare Listen aus den Betreuungsplan-Slots
    // (Plan/Ist/Abgleich). Desktop-only im Akkordeon; mobil über den
    // Betreuungsplan-Eintrag erreichbar, die Seite selbst hat einen
    // Zurück-Button.
    href: "/lists",
    label: "Tageslisten",
    legacyPrefixes: [],
    showInMobileNav: false,
    mobileParentHref: "/betreuungsplan",
  },
  {
    href: "/calendar-periods",
    label: "Kalenderzeiträume",
    legacyPrefixes: [],
    showInMobileNav: false,
    mobileParentHref: "/betreuungsplan",
  },
];

function matchesPathPrefix(pathname: string, prefix: string): boolean {
  return pathname === prefix || pathname.startsWith(`${prefix}/`);
}

export function getActivePlanningSubPageHref(
  pathname: string,
): PlanningPageHref | null {
  for (const page of PLANNING_SUB_PAGES) {
    if (matchesPathPrefix(pathname, page.href)) return page.href;
    if (
      page.legacyPrefixes.some((prefix) => matchesPathPrefix(pathname, prefix))
    ) {
      return page.href;
    }
  }
  return null;
}

export function isPlanningPath(pathname: string): boolean {
  return getActivePlanningSubPageHref(pathname) !== null;
}

export function isPlanningPageHref(href: string): href is PlanningPageHref {
  return PLANNING_SUB_PAGES.some((page) => page.href === href);
}

/**
 * Paths that should activate one entry in the flattened mobile navigation.
 * This includes that page's legacy redirects and any desktop-only children.
 */
export function getPlanningMobileActivePaths(href: PlanningPageHref): string[] {
  const paths: string[] = [];
  for (const page of PLANNING_SUB_PAGES) {
    if (page.href === href || page.mobileParentHref === href) {
      paths.push(page.href, ...page.legacyPrefixes);
    }
  }
  return paths;
}
