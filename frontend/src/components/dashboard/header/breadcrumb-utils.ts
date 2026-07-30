// Breadcrumb utilities for header navigation
// Extracted to reduce cognitive complexity in header.tsx

import {
  getActivePlanningSubPageHref,
  PLANNING_SUB_PAGES,
} from "~/lib/planning-navigation";
import {
  DATABASE_SECTION,
  getActiveDatabaseSubPage,
  getActiveParentSubPage,
  PARENT_SECTION,
  PLANNING_SECTION,
} from "~/lib/section-navigation";

const exactPageTitles: Record<string, string> = {
  "/students/search": "Kindersuche",
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
  {
    basePath: "/rooms/",
    rootPath: "/rooms",
    title: "Raum Details",
  },
];

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
  const databasePage = getActiveDatabaseSubPage(pathname);
  if (databasePage) {
    const isDeepPage = pathname !== databasePage.href;
    return {
      sectionLabel: DATABASE_SECTION.label,
      sectionHref: DATABASE_SECTION.href,
      pageLabel: databasePage.label,
      pageHref: isDeepPage ? databasePage.href : undefined,
      deepLabel: isDeepPage ? getSubPageLabel(pathname) : undefined,
    };
  }

  const planningHref = getActivePlanningSubPageHref(pathname);
  if (planningHref) {
    const page = PLANNING_SUB_PAGES.find(
      (entry) => entry.href === planningHref,
    );
    if (page) {
      return {
        sectionLabel: PLANNING_SECTION.label,
        sectionHref: PLANNING_SECTION.href,
        pageLabel: page.label,
      };
    }
  }

  const parentPage = getActiveParentSubPage(pathname);
  if (parentPage && parentPage.href !== PARENT_SECTION.href) {
    return {
      sectionLabel: PARENT_SECTION.label,
      sectionHref: PARENT_SECTION.href,
      pageLabel: parentPage.label,
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

  if (isEnrollmentPath(pathname)) return getEnrollmentPageTitle(pathname);

  if (pathname.startsWith("/parents/children")) {
    return pathname === "/parents/children" ? "Meine Kinder" : "Kinderprofil";
  }

  // Handle main routes
  return getMainRouteTitle(pathname);
}

function getStudentPageTitle(pathname: string): string {
  if (pathname.includes("/feedback-history")) return "Feedback Historie";
  if (pathname.includes("/room-history")) return "Anwesenheitsprotokoll";
  if (pathname.includes("/change-history")) return "Änderungsverlauf";
  return "Kinder Details";
}

function getDetailRouteTitle(pathname: string): string | null {
  const route = detailRouteTitles.find(
    ({ basePath, rootPath }) =>
      pathname.startsWith(basePath) && pathname !== rootPath,
  );
  return route?.title ?? null;
}

function getMainRouteTitle(pathname: string): string {
  const mainRoutes: Record<string, string> = {
    "/dashboard": "Home",
    "/": "Home",
    "/parents": "Start",
    "/ogs-groups": "Meine Gruppe",
    "/active-supervisions": "Aktuelle Aufsicht",
    "/staff": "Mitarbeiter",
    "/rooms": "Räume",
    "/activities": "Aktivitäten",
    "/reminders": "Erinnerungen",
    "/substitutions": "Gruppenzugriff",
    "/calendar": "Mein Kalender",
    "/day-log": "Tagesauswertung",
    "/info-displays": "Info-Displays",
    // Die Planungsseiten selbst kommen aus PLANNING_SUB_PAGES
    // (getSectionBreadcrumb); /planung ist nur der Redirect-Frame und behält
    // einen Eintrag, damit während des Client-Redirects kein falscher Titel
    // aufblitzt.
    "/planung": "Planung",
    // Die beiden Sektions-Hubs; ihre Unterseiten kommen aus den Katalogen.
    "/database": "Datenverwaltung",
    "/eltern": "Eltern",
    "/emergency": "Notfall",
    "/settings": "Einstellungen",
    "/profile": "Profil",
    "/admin/enrollments": "Überblick",
    "/admin/enrollments/change-requests": "Änderungsanfragen",
    "/enrollment-phases": "Anmeldephasen",
    "/care-offerings": "Betreuungsangebote",
    "/enrollment-form": "Anmeldeformulare",
    "/time-tracking": "Zeiterfassung",
    "/suggestions": "Feedback",
    "/operator/suggestions": "Vorschläge",
    "/operator/announcements": "Ankündigungen",
    "/operator/organizations": "Träger",
    "/operator/schools": "Schulen",
    "/operator/accounts": "Konten",
    "/operator/devices": "Geräte",
    "/operator/persons": "Personen",
    "/operator/unregistered-tags": "Unbekannte RFID",
    "/operator/operators": "Operatoren",
    "/parents/messages": "Nachrichten",
    "/parents/news": "Neuigkeiten",
    "/parents/meal-plan": "Essensplan",
  };

  return mainRoutes[pathname] ?? "Home";
}

/**
 * Get human-readable label for sub-pages in breadcrumbs
 */
export function getSubPageLabel(pathname: string): string {
  const segments = pathname.split("/").filter(Boolean);
  const lastSegment = segments.at(-1);

  const subPageLabels: Record<string, string> = {
    import: "Importieren",
    create: "Erstellen",
    edit: "Bearbeiten",
    details: "Details",
    permissions: "Berechtigungen",
  };

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
  return "Kindersuche";
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
  isRoomDetailPage: boolean;
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

  const isRoomDetailPage =
    pathname.startsWith("/rooms/") && pathname !== "/rooms";

  const isEnrollmentPage = isEnrollmentPath(pathname);

  return {
    isStudentDetailPage,
    isStudentHistoryPage,
    isStaffDetailPage,
    isRoomDetailPage,
    isEnrollmentPage,
  };
}

function isEnrollmentPath(pathname: string): boolean {
  return (
    pathname.startsWith("/admin/enrollments") ||
    pathname.startsWith("/enrollment-phases") ||
    pathname.startsWith("/care-offerings") ||
    pathname.startsWith("/enrollment-form")
  );
}

function getEnrollmentPageTitle(pathname: string): string {
  if (pathname.startsWith("/admin/enrollments/change-requests")) {
    return "Änderungsanfragen";
  }
  if (pathname.startsWith("/admin/enrollments/phases/")) {
    return "Anmeldephase";
  }
  if (pathname.startsWith("/admin/enrollments/")) {
    return "Anmeldung";
  }
  if (pathname.startsWith("/admin/enrollments")) return "Überblick";
  if (pathname.startsWith("/enrollment-phases")) return "Anmeldephasen";
  if (pathname.startsWith("/care-offerings")) return "Betreuungsangebote";
  if (pathname.startsWith("/enrollment-form")) return "Anmeldeformulare";
  return "Anmeldungen";
}
