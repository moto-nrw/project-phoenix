// Breadcrumb utilities for header navigation
// Extracted to reduce cognitive complexity in header.tsx

/**
 * Get page title based on pathname
 */
export function getPageTitle(pathname: string): string {
  // Check for /students/search first before other /students/ paths
  if (pathname === "/students/search") {
    return "Kindersuche";
  }

  // Handle student detail pages
  if (pathname.startsWith("/students/") && pathname !== "/students") {
    return getStudentPageTitle(pathname);
  }

  // Handle staff detail pages
  if (pathname.startsWith("/staff/") && pathname !== "/staff") {
    return "Mitarbeiter Details";
  }

  // Handle room detail pages
  if (pathname.startsWith("/rooms/") && pathname !== "/rooms") {
    return "Raum Details";
  }

  // Handle database sub-pages
  if (pathname.startsWith("/database/")) {
    return getDatabasePageTitle(pathname);
  }

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
  return "Kinder Details";
}

function getDatabasePageTitle(pathname: string): string {
  const databasePages: Record<string, string> = {
    activities: "Aktivitäten",
    groups: "Gruppen",
    students: "Kinder",
    personal: "Personal",
    rooms: "Räume",
    roles: "Rollen",
    devices: "Geräte",
    permissions: "Berechtigungen",
  };

  for (const [key, title] of Object.entries(databasePages)) {
    if (pathname.includes(`/${key}`)) return title;
  }
  return "Datenbank";
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
    "/substitutions": "Vertretungen",
    "/timetables": "Betreuungsplan",
    "/database": "Datenverwaltung",
    "/emergency": "Notfall",
    "/settings": "Einstellungen",
    "/profile": "Profil",
    "/admin/enrollments": "Überblick",
    "/enrollment-phases": "Anmeldephasen",
    "/care-offerings": "Betreuungsangebote",
    "/enrollment-form": "Anmeldeformulare",
    "/time-tracking": "Zeiterfassung",
    "/operator/suggestions": "Vorschläge",
    "/operator/announcements": "Ankündigungen",
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
  isDatabaseSubPage: boolean;
  isDatabaseDeepPage: boolean;
  isEnrollmentPage: boolean;
}

export function getPageTypeInfo(pathname: string): PageTypeInfo {
  const isStudentPath = pathname.startsWith("/students/");
  const isStudentDetailPage =
    isStudentPath &&
    pathname !== "/students" &&
    pathname !== "/students/search" &&
    !pathname.includes("/feedback-history") &&
    !pathname.includes("/room-history");

  const isStudentHistoryPage =
    isStudentPath &&
    (pathname.includes("/feedback-history") ||
      pathname.includes("/room-history"));

  const isStaffDetailPage =
    pathname.startsWith("/staff/") && pathname !== "/staff";

  const isRoomDetailPage =
    pathname.startsWith("/rooms/") && pathname !== "/rooms";

  const isDatabaseSubPage =
    pathname.startsWith("/database/") && pathname !== "/database";

  const isDatabaseDeepPage = pathname.split("/").length > 3;
  const isEnrollmentPage = isEnrollmentPath(pathname);

  return {
    isStudentDetailPage,
    isStudentHistoryPage,
    isStaffDetailPage,
    isRoomDetailPage,
    isDatabaseSubPage,
    isDatabaseDeepPage,
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
