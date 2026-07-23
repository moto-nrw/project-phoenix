// Breadcrumb utilities for header navigation
// Extracted to reduce cognitive complexity in header.tsx

const exactPageTitles: Record<string, string> = {
  "/staff/dienstplan": "Dienstplan",
  "/admin/guardian-approvals": "Konto-Anfragen",
  "/admin/change-requests": "Änderungsanfragen",
  "/parent-announcements": "Elternmitteilungen",
  "/meal-plan": "Essensplan",
  "/students/search": "Kindersuche",
};

const segmentPageTitles: Record<string, string> = {
  "/messages": "Nachrichten",
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

export function getPageTitle(pathname: string): string {
  const exactTitle = exactPageTitles[pathname];
  if (exactTitle) return exactTitle;

  const segmentTitle = getSegmentPageTitle(pathname);
  if (segmentTitle) return segmentTitle;

  // Handle student detail pages
  if (pathname.startsWith("/students/") && pathname !== "/students") {
    return getStudentPageTitle(pathname);
  }

  const detailTitle = getDetailRouteTitle(pathname);
  if (detailTitle) return detailTitle;

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

function getSegmentPageTitle(pathname: string): string | null {
  for (const [basePath, title] of Object.entries(segmentPageTitles)) {
    if (matchesPathSegment(pathname, basePath)) return title;
  }
  return null;
}

function getDetailRouteTitle(pathname: string): string | null {
  const route = detailRouteTitles.find(
    ({ basePath, rootPath }) =>
      pathname.startsWith(basePath) && pathname !== rootPath,
  );
  return route?.title ?? null;
}

function matchesPathSegment(pathname: string, basePath: string): boolean {
  return pathname === basePath || pathname.startsWith(`${basePath}/`);
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
    exports: "Exporte",
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
    "/staff/dienstplan": "Dienstplan",
    "/rooms": "Räume",
    "/activities": "Aktivitäten",
    "/reminders": "Erinnerungen",
    "/substitutions": "Gruppenzugriff",
    "/calendar-periods": "Kalenderzeiträume",
    // Die drei Planungsbereiche (Planung-Redesign, docs/planung-redesign/
    // docs/03 Abschnitt 5); die Redirect-Frames behalten Einträge, damit
    // während des Client-Redirects kein falscher Titel aufblitzt.
    "/betreuungsplan": "Betreuungsplan",
    "/dienstplan": "Dienstplan",
    "/vertretung": "Vertretung",
    "/planung": "Planung",
    "/timetables": "Betreuungsplan",
    "/vertretungsplan": "Vertretung",
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
    pathname.startsWith("/staff/") &&
    pathname !== "/staff" &&
    pathname !== "/staff/dienstplan";

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
