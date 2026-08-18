// Breadcrumb utilities for header navigation
// Extracted to reduce cognitive complexity in header.tsx

import { getActivePlanningSubPage } from "~/lib/planning-navigation";
import {
  DATABASE_SECTION,
  ENROLLMENT_SECTION,
  getActiveDatabaseSubPage,
  getActiveEnrollmentSubPage,
  getActiveParentSubPage,
  PARENT_SECTION,
  PLANNING_SECTION,
  STAFF_FLAT_PAGES,
  type SectionRoot,
  type SectionSubPage,
} from "~/lib/section-navigation";

/**
 * Pfade, die vor jeder Präfixregel greifen müssen. /students/search liegt
 * unter /students/, würde also sonst als Kinder-Detailseite gelesen.
 */
const exactPageTitles: Record<string, string> = {
  [STAFF_FLAT_PAGES.studentSearch.href]: STAFF_FLAT_PAGES.studentSearch.label,
};

const detailRouteTitles: Array<{
  basePath: string;
  rootPath: string;
  title: string;
}> = [
  {
    basePath: "/staff/",
    rootPath: "/staff",
    title: "Mitarbeiter Details",
  },
];

/**
 * Titel der flachen Hauptrouten — also aller Seiten, die zu KEINEM
 * Navigationskatalog gehören. Alles, was in einem Katalog steht
 * (section-navigation.ts, planning-navigation.ts), holt seinen Titel von dort
 * und darf hier NICHT nochmal auftauchen: eine zweite Kopie läuft irgendwann
 * auseinander. Fehlt ein Eintrag, zeigt die Breadcrumb stillschweigend
 * "Home" — `navigation-sync.test.ts` fängt genau das für die Katalogseiten ab.
 */
const mainRoutes: Record<string, string> = {
  // Die flachen Mitarbeiter-Seiten stehen im gemeinsamen Katalog, aus dem
  // auch die Seitenleiste ihre Einträge baut.
  ...Object.fromEntries(
    Object.values(STAFF_FLAT_PAGES).map((page) => [page.href, page.label]),
  ),
  "/": "Home",
  "/parents": "Start",
  // Die beiden Akkordeon-Bereiche. Ihre Beschriftung in der Seitenleiste ist
  // dynamisch (Ein-/Mehrzahl je nach Anzahl, plus Anwesenheitszähler) und
  // taugt deshalb nicht als gemeinsame Quelle.
  "/ogs-groups": "Meine Gruppe",
  "/active-supervisions": "Aktuelle Aufsicht",
  // Ohne Navigationseintrag, nur über Verlinkung erreichbar.
  "/reminders": "Erinnerungen",
  // Abwesenheits-Übersicht (#2288): bewusst ohne Seitenleisten-Eintrag,
  // erreichbar über das Aktionsmenü der Kindersuche.
  "/absences": "Abwesenheiten",
  "/profile": "Profil",
  // Die Planungsseiten selbst kommen aus PLANNING_SUB_PAGES
  // (getSectionBreadcrumb); /planung ist nur der Redirect-Frame und behält
  // einen Eintrag, damit während des Client-Redirects kein falscher Titel
  // aufblitzt.
  "/planung": PLANNING_SECTION.label,
  // Die Sektions-Hubs; ihre Unterseiten kommen aus den Katalogen.
  [DATABASE_SECTION.href]: DATABASE_SECTION.label,
  [PARENT_SECTION.href]: PARENT_SECTION.label,
  // Operator-Seiten setzen ihren Titel selbst per useSetBreadcrumb. Diese
  // Einträge sind der Wert für den ersten Frame davor und müssen deshalb
  // wörtlich mit dem Seitentitel übereinstimmen, sonst blitzt beim Laden
  // kurz ein anderes Wort auf. `operator_page_titles` in
  // navigation-sync.test.ts hält beide Seiten zusammen.
  "/operator/suggestions": "Feedback",
  "/operator/announcements": "Ankündigungen",
  "/operator/organizations": "Träger",
  "/operator/schools": "Schulen",
  "/operator/accounts": "Konten",
  "/operator/devices": "Geräte",
  "/operator/persons": "Personen",
  "/operator/unregistered-tags": "Unbekannte RFID",
  "/operator/operators": "Operatoren",
  // Elternportal: nur der deutsche Rückfallwert. Die Kopfzeile überschreibt
  // ihn im Elternmodus mit dem übersetzten parentNav-Eintrag.
  "/parents/messages": "Nachrichten",
  "/parents/news": "Neuigkeiten",
  "/parents/meal-plan": "Essensplan",
  "/parents/feedback": "Feedback",
};

const subPageLabels: Record<string, string> = {
  import: "Importieren",
  create: "Erstellen",
  edit: "Bearbeiten",
  details: "Details",
  permissions: "Berechtigungen",
  "opening-balances": "Eröffnungssalden",
  // Klassenlisteneinträge (#2382): Kinder ohne OGS-Datensatz.
  "class-list": "Klassenliste",
};

/**
 * Zweistufige Breadcrumb einer gruppierten Navigationssektion
 * (Datenverwaltung, Planung, Eltern) — plus optional eine dritte Stufe für
 * Unterseiten wie /database/students/import.
 */
export interface SectionBreadcrumbInfo {
  readonly sectionLabel: string;
  /** Fehlt, wenn die Sektion keine Hub-Seite hat (Planung). */
  readonly sectionHref?: string;
  readonly pageLabel: string;
  /** Nur gesetzt, wenn eine dritte Stufe folgt; macht die Mittelstufe zum Link. */
  readonly pageHref?: string;
  readonly deepLabel?: string;
}

/**
 * Alle Bereiche, die eine zweistufige Breadcrumb bekommen, in Prüfreihenfolge.
 * Eine neue Sektion braucht genau eine Zeile hier plus ihren Katalog in
 * section-navigation.ts — keine eigene if-Kaskade mehr.
 */
const BREADCRUMB_SECTIONS: readonly {
  readonly root: SectionRoot;
  readonly getActivePage: (pathname: string) => SectionSubPage | null;
  /** Dritte Stufe für Unterseiten wie /database/students/import. */
  readonly hasDeepPages?: boolean;
}[] = [
  {
    root: DATABASE_SECTION,
    getActivePage: getActiveDatabaseSubPage,
    hasDeepPages: true,
  },
  { root: PLANNING_SECTION, getActivePage: getActivePlanningSubPage },
  { root: PARENT_SECTION, getActivePage: getActiveParentSubPage },
];

/**
 * Ordnet einen Pfad seiner Navigationssektion zu. Labels kommen aus denselben
 * Katalogen, aus denen die Seitenleiste rendert, damit Seitenleisten-Eintrag
 * und Breadcrumb nicht auseinanderlaufen können.
 *
 * Die Hub-Seiten selbst (/database, /eltern) liefern bewusst `null`: sie zeigen
 * nur ihren Sektionsnamen, keine Breadcrumb auf sich selbst.
 */
export function getSectionBreadcrumb(
  pathname: string,
): SectionBreadcrumbInfo | null {
  for (const { root, getActivePage, hasDeepPages } of BREADCRUMB_SECTIONS) {
    const page = getActivePage(pathname);
    if (!page || page.href === root.href) continue;

    const isDeepPage = Boolean(hasDeepPages) && pathname !== page.href;
    return {
      sectionLabel: root.label,
      sectionHref: root.href,
      pageLabel: page.label,
      pageHref: isDeepPage ? page.href : undefined,
      deepLabel: isDeepPage ? getSubPageLabel(pathname) : undefined,
    };
  }

  return null;
}

export function getPageTitle(pathname: string): string {
  const exactTitle = exactPageTitles[pathname];
  if (exactTitle) return exactTitle;

  // Handle student detail pages
  if (pathname.startsWith("/students/") && pathname !== "/students") {
    return getStudentPageTitle(pathname);
  }

  // Sektionsseiten tragen das Label ihres Navigationseintrags. Muss VOR der
  // Detailrouten-Prüfung laufen: /staff/dienstplan ist die Planungsseite
  // "Dienstplan" und nicht die Detailansicht einer Mitarbeiterin.
  const section = getSectionBreadcrumb(pathname);
  if (section) return section.deepLabel ?? section.pageLabel;

  // Unbekannte Unterseite der Datenverwaltung: den Bereichsnamen zeigen statt
  // in den "Home"-Fallback zu rutschen.
  if (pathname.startsWith("/database/")) return DATABASE_SECTION.label;

  const detailTitle = getDetailRouteTitle(pathname);
  if (detailTitle) return detailTitle;

  const enrollmentPage = getActiveEnrollmentSubPage(pathname);
  if (enrollmentPage) return getEnrollmentPageTitle(pathname, enrollmentPage);

  if (pathname.startsWith("/parents/children")) {
    return pathname === "/parents/children" ? "Meine Kinder" : "Kinderprofil";
  }

  // Handle main routes
  return getMainRouteTitle(pathname);
}

function getStudentPageTitle(pathname: string): string {
  return getHistoryType(pathname) || "Kinder Details";
}

function getDetailRouteTitle(pathname: string): string | null {
  const route = detailRouteTitles.find(
    ({ basePath, rootPath }) =>
      pathname.startsWith(basePath) && pathname !== rootPath,
  );
  return route?.title ?? null;
}

function getMainRouteTitle(pathname: string): string {
  return mainRoutes[pathname] ?? "Home";
}

/**
 * Get human-readable label for sub-pages in breadcrumbs
 */
export function getSubPageLabel(pathname: string): string {
  const segments = pathname.split("/").filter(Boolean);
  const lastSegment = segments.at(-1);

  if (!lastSegment) return "Unbekannt";
  return (
    subPageLabels[lastSegment] ??
    lastSegment.charAt(0).toUpperCase() + lastSegment.slice(1)
  );
}

/**
 * Determine breadcrumb context based on referrer
 */
export function getBreadcrumbLabel(referrer: string): string {
  if (referrer.startsWith("/ogs-groups")) return "Meine Gruppe";
  if (referrer.startsWith("/active-supervisions")) return "Aktuelle Aufsicht";
  // Drill-in from a room detail (legacy /rooms/{id} subpage OR the new
  // /rooms?room={id} modal flow, see #1374). The breadcrumb has to
  // point back to the entry path in both cases so the header label and
  // the active sidebar entry agree with how the user actually got here.
  if (referrer.startsWith("/rooms/") || referrer.startsWith("/rooms?"))
    return "Räume";
  return "Alle Kinder";
}

/**
 * Determine history type from pathname
 */
export function getHistoryType(pathname: string): string {
  if (pathname.includes("/feedback-history")) return "Feedback Historie";
  if (pathname.includes("/room-history")) return "Anwesenheitsprotokoll";
  if (pathname.includes("/change-history")) return "Änderungsverlauf";
  return "";
}

/**
 * Check page type from pathname
 */
export interface PageTypeInfo {
  isStudentDetailPage: boolean;
  isStudentHistoryPage: boolean;
  isStaffDetailPage: boolean;
  isEnrollmentPage: boolean;
}

export function getPageTypeInfo(pathname: string): PageTypeInfo {
  const isStudentPath = pathname.startsWith("/students/");
  const isStudentDetailPage =
    isStudentPath &&
    pathname !== "/students" &&
    pathname !== "/students/search" &&
    !pathname.includes("/feedback-history") &&
    !pathname.includes("/room-history") &&
    !pathname.includes("/change-history");

  const isStudentHistoryPage =
    isStudentPath &&
    (pathname.includes("/feedback-history") ||
      pathname.includes("/room-history") ||
      pathname.includes("/change-history"));

  const isStaffDetailPage =
    pathname.startsWith("/staff/") &&
    pathname !== "/staff" &&
    pathname !== "/staff/dienstplan";

  const isEnrollmentPage = getActiveEnrollmentSubPage(pathname) !== null;

  return {
    isStudentDetailPage,
    isStudentHistoryPage,
    isStaffDetailPage,
    isEnrollmentPage,
  };
}

/**
 * Die Anmeldungen-Seiten tragen die Labels ihres Katalogeintrags. Nur die
 * beiden Detailrouten unterhalb des Hubs haben keinen eigenen Eintrag und
 * brauchen deshalb eine Sonderregel.
 */
function getEnrollmentPageTitle(
  pathname: string,
  page: SectionSubPage,
): string {
  if (pathname.startsWith("/admin/enrollments/phases/")) return "Anmeldephase";
  if (page.href === ENROLLMENT_SECTION.href && pathname !== page.href) {
    return "Anmeldung";
  }
  return page.label;
}
