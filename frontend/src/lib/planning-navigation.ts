import { matchesPathPrefix } from "~/lib/section-navigation";

export type PlanningPageHref =
  | "/betreuungsplan"
  | "/dienstplan"
  | "/vertretung"
  | "/lists"
  | "/wochenprotokolle"
  | "/calendar-periods"
  | "/payroll";

export interface PlanningSubPage {
  readonly href: PlanningPageHref;
  readonly label: string;
  readonly showInMobileNav: boolean;
  readonly nonAdminPermission?: string;
  /**
   * Die Seite gehört zum Betreuungsplan und verschwindet, wenn die Schule
   * ihn ausschaltet (`timetable.enabled`). Dienstplan und Abrechnung hängen
   * nicht daran: sie folgen ihren Modulen (#4894).
   */
  readonly requiresTimetable: boolean;
}

/**
 * Single source of truth for canonical planning navigation.
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
    showInMobileNav: true,
    requiresTimetable: true,
    // Der Planungsbereich gehört der Leitung: wer den Plan bearbeiten darf
    // (schedules:manage, das Recht der Schreibrouten), sieht den Eintrag, ob
    // Admin oder eine eigene Leitungsrolle der Schule (#3469). Alle anderen
    // erreichen die Leseansicht (#2283) als Tab "Betreuungsplan" in "Mein
    // Kalender" (eine Kalenderfläche), nicht über die Planungsgruppe.
    nonAdminPermission: "schedules:manage",
  },
  {
    href: "/dienstplan",
    label: "Dienstplan",
    showInMobileNav: true,
    requiresTimetable: false,
    // Schichten schreibt das Backend mit time_tracking:manage; dieselbe
    // Schranke öffnet den Eintrag und das Bearbeiten in der Ansicht.
    nonAdminPermission: "time_tracking:manage",
  },
  {
    // „Vertretungsplan", nicht „Terminvertretungen": neben Betreuungsplan und
    // Dienstplan die dritte Planung. Die Tagesübersicht im Tagesbetrieb heißt
    // „Übergaben" (#4778), damit kein Navigationseintrag mit „Vertretung"
    // daneben steht (#2826).
    href: "/vertretung",
    label: "Vertretungsplan",
    showInMobileNav: true,
    requiresTimetable: true,
    nonAdminPermission: "schedules:manage",
  },
  {
    // Tageslisten (#1565): druckbare Listen aus den Betreuungsplan-Slots
    // (Plan/Ist/Abgleich). Die Seite selbst hat einen Zurück-Button. Die
    // Listen selbst lesen mit schedules:read und users:read; als Werkzeug der
    // Planung hängt der Eintrag am Planungsrecht, damit er nicht jeder
    // Betreuungskraft in einer Gruppe „Planung" erscheint.
    href: "/lists",
    label: "Tageslisten",
    showInMobileNav: true,
    requiresTimetable: true,
    nonAdminPermission: "schedules:manage",
  },
  {
    // Wochenprotokolle (#4769): die Protokolle aller Gruppen für die Leitung,
    // die alle Gruppen führt (groups:manage). Die Gruppenleitung öffnet die
    // Protokolle ihrer Gruppe von der Gruppenseite aus.
    href: "/wochenprotokolle",
    label: "Wochenprotokolle",
    showInMobileNav: true,
    requiresTimetable: false,
    nonAdminPermission: "groups:manage",
  },
  {
    // Die Seite verwaltet Schuljahr, Halbjahre, Ferien und Schließtage. Der
    // Fachbegriff „Kalenderzeitraum" bleibt im Formularfeld, das einen davon
    // auswählt; in der Navigation sagt der Name, was drin ist (#2826).
    // Zeiträume und Schließtage schreibt das Backend mit den schedules-
    // Rechten; schedules:manage steht für die Leitung, die sie pflegt.
    href: "/calendar-periods",
    label: "Schuljahr und Kalender",
    showInMobileNav: true,
    requiresTimetable: false,
    nonAdminPermission: "schedules:manage",
  },
  {
    // Abrechnung war ein eigener flacher Eintrag; sie gehört inhaltlich zur
    // Planung (Lohnabrechnung aus Dienstplan und Zeiterfassung) und steht
    // deshalb hier. Mobil hatte sie bisher keinen Eintrag; als Planungsseite
    // bekommt sie einen, wie jede andere auch (siehe Regel oben).
    href: "/payroll",
    label: "Abrechnung",
    showInMobileNav: true,
    requiresTimetable: false,
    nonAdminPermission: "config:manage",
  },
];

export function getActivePlanningSubPage(
  pathname: string,
): PlanningSubPage | null {
  for (const page of PLANNING_SUB_PAGES) {
    if (matchesPathPrefix(pathname, page.href)) return page;
  }
  return null;
}

export function getActivePlanningSubPageHref(
  pathname: string,
): PlanningPageHref | null {
  return getActivePlanningSubPage(pathname)?.href ?? null;
}

export function isPlanningPageHref(href: string): href is PlanningPageHref {
  return PLANNING_SUB_PAGES.some((page) => page.href === href);
}
