import { matchesPathPrefix } from "~/lib/section-navigation";

export type PlanningPageHref =
  | "/betreuungsplan"
  | "/dienstplan"
  | "/vertretung"
  | "/lists"
  | "/calendar-periods"
  | "/calendar";

export interface PlanningSubPage {
  readonly href: PlanningPageHref;
  readonly label: string;
  readonly legacyPrefixes: readonly string[];
  readonly showInMobileNav: boolean;
  readonly nonAdminPermission?: string;
  /**
   * Die Seite hängt NICHT am Schalter timetable.enabled. Bisher stand diese
   * Ausnahme als Pfad-Liste in Seitenleiste und mobiler Navigation, also
   * zweimal — wer eine Seite ergänzte, traf meist nur eine Stelle.
   */
  readonly independentOfTimetable?: boolean;
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
    href: "/vertretung",
    label: "Vertretung",
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
    // „Kalenderzeiträume" neben „Mein Kalender" waren zwei Namen mit
    // demselben Wortstamm für zwei verschiedene Dinge. Der Zeitraum heißt
    // jetzt schlicht „Zeiträume".
    href: "/calendar-periods",
    label: "Zeiträume",
    legacyPrefixes: [],
    showInMobileNav: true,
    independentOfTimetable: true,
  },
  {
    // „Mein Kalender" hieß anders als der Bereich, in dem er steht, und war
    // der einzige Planungseintrag außerhalb der Planung. Jetzt heißt er
    // „Kalender" und steht hier. Er hängt an calendar:own, nicht an
    // timetable.enabled.
    href: "/calendar",
    label: "Kalender",
    legacyPrefixes: [],
    showInMobileNav: true,
    nonAdminPermission: "calendar:own",
    independentOfTimetable: true,
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

/**
 * Bleibt die Seite auch bei ausgeschaltetem timetable.enabled stehen?
 */
export function isPlanningPageIndependentOfTimetable(href: string): boolean {
  return (
    PLANNING_SUB_PAGES.find((page) => page.href === href)
      ?.independentOfTimetable === true
  );
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
