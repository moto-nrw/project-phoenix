import { matchesPathPrefix } from "~/lib/section-navigation";

export type PlanningPageHref =
  | "/betreuungsplan"
  | "/dienstplan"
  | "/vertretung"
  | "/lists"
  | "/calendar-periods"
  | "/payroll";

export interface PlanningSubPage {
  readonly href: PlanningPageHref;
  readonly label: string;
  readonly legacyPrefixes: readonly string[];
  readonly showInMobileNav: boolean;
  readonly nonAdminPermission?: string;
}

/**
 * Single source of truth for planning navigation and its legacy redirects.
 *
 * Jede Planungsseite steht auch in der flachen mobilen Navigation. Tageslisten
 * und Kalenderzeiträume waren dort ausgenommen, mit der Begründung, sie seien
 * "über den Betreuungsplan-Eintrag erreichbar" — das traf nicht zu: es gibt
 * keinen Verweis vom Betreuungsplan dorthin. Kalenderzeiträume erreichte man
 * nur beiläufig über den Zeitraum-Auswähler, Tageslisten ausschließlich über
 * einen Link in der Datenverwaltung. Beide Seiten funktionieren mobil (das
 * Desktop-Gate von Kalenderzeiträume fiel mit #2033), also gehören sie auch
 * mobil in die Navigation.
 */
export const PLANNING_SUB_PAGES: readonly PlanningSubPage[] = [
  {
    href: "/betreuungsplan",
    label: "Betreuungsplan",
    legacyPrefixes: ["/timetables"],
    showInMobileNav: true,
    // Leseansicht (#2283): Nicht-Admins erreichen den Wochenplan als Tab
    // "Betreuungsplan" in "Mein Kalender" (eine Kalenderfläche) — deshalb
    // hier bewusst KEINE nonAdminPermission; der Planungsbereich in der
    // Navigation bleibt Admins vorbehalten.
  },
  {
    href: "/dienstplan",
    label: "Dienstplan",
    legacyPrefixes: ["/staff/dienstplan"],
    showInMobileNav: true,
  },
  {
    // „Vertretungsplan", nicht „Terminvertretungen": neben Betreuungsplan und
    // Dienstplan die dritte Planung; die Endung zeigt die Grenze zur
    // Tagesübersicht „Vertretungen" im Tagesbetrieb (#2826).
    href: "/vertretung",
    label: "Vertretungsplan",
    legacyPrefixes: ["/vertretungsplan"],
    showInMobileNav: true,
  },
  {
    // Tageslisten (#1565): druckbare Listen aus den Betreuungsplan-Slots
    // (Plan/Ist/Abgleich). Die Seite selbst hat einen Zurück-Button.
    href: "/lists",
    label: "Tageslisten",
    legacyPrefixes: [],
    showInMobileNav: true,
  },
  {
    // Die Seite verwaltet Schuljahr, Halbjahre, Ferien und Schließtage. Der
    // Fachbegriff „Kalenderzeitraum" bleibt im Formularfeld, das einen davon
    // auswählt; in der Navigation sagt der Name, was drin ist (#2826).
    href: "/calendar-periods",
    label: "Schuljahr und Ferien",
    legacyPrefixes: [],
    showInMobileNav: true,
  },
  {
    // Abrechnung war ein eigener flacher Eintrag; sie gehört inhaltlich zur
    // Planung (Lohnabrechnung aus Dienstplan und Zeiterfassung) und steht
    // deshalb hier. Mobil hatte sie bisher keinen Eintrag; als Planungsseite
    // bekommt sie einen, wie jede andere auch (siehe Regel oben).
    href: "/payroll",
    label: "Abrechnung",
    legacyPrefixes: [],
    showInMobileNav: true,
    nonAdminPermission: "config:manage",
  },
];

export function getActivePlanningSubPage(
  pathname: string,
): PlanningSubPage | null {
  for (const page of PLANNING_SUB_PAGES) {
    if (matchesPathPrefix(pathname, page.href)) return page;
    if (
      page.legacyPrefixes.some((prefix) => matchesPathPrefix(pathname, prefix))
    ) {
      return page;
    }
  }
  return null;
}

export function getActivePlanningSubPageHref(
  pathname: string,
): PlanningPageHref | null {
  return getActivePlanningSubPage(pathname)?.href ?? null;
}

export function isPlanningPath(pathname: string): boolean {
  return getActivePlanningSubPageHref(pathname) !== null;
}

export function isPlanningPageHref(href: string): href is PlanningPageHref {
  return PLANNING_SUB_PAGES.some((page) => page.href === href);
}

/**
 * Paths that should activate one entry in the flattened mobile navigation:
 * the page itself plus its legacy redirects.
 */
export function getPlanningMobileActivePaths(href: PlanningPageHref): string[] {
  const page = PLANNING_SUB_PAGES.find((entry) => entry.href === href);
  return page ? [page.href, ...page.legacyPrefixes] : [];
}
