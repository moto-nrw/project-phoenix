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

  // Handle room detail pages
  if (pathname.startsWith("/rooms/") && pathname !== "/rooms") {
    return "Raum Details";
  }

  // Handle database sub-pages
  if (pathname.startsWith("/database/")) {
    return getDatabasePageTitle(pathname);
  }

  // Handle main routes
  return getMainRouteTitle(pathname);
}

function getStudentPageTitle(pathname: string): string {
  if (pathname.includes("/feedback_history")) return "Feedback Historie";
  if (pathname.includes("/mensa_history")) return "Mensa Historie";
  if (pathname.includes("/room_history")) return "Raum Historie";
  return "Schüler Details";
}

function getDatabasePageTitle(pathname: string): string {
  const databasePages: Record<string, string> = {
    activities: "Aktivitäten",
    groups: "Gruppen",
    students: "Kinder",
    teachers: "Betreuer",
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
    "/ogs-groups": "Meine Gruppe",
    "/active-supervisions": "Aktuelle Aufsicht",
    "/staff": "Mitarbeiter",
    "/students": "Schüler",
    "/rooms": "Räume",
    "/activities": "Aktivitäten",
    "/statistics": "Statistiken",
    "/substitutions": "Vertretungen",
    "/database": "Datenverwaltung",
    "/settings": "Einstellungen",
    "/invitations": "Einladungen",
    "/time-tracking": "Zeiterfassung",
    "/borndal_feedback": "Borndal Feedback",
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
    "csv-import": "CSV-Import",
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
  return "Kindersuche";
}

/**
 * Determine history type from pathname
 */
export function getHistoryType(pathname: string): string {
  if (pathname.includes("/feedback_history")) return "Feedback Historie";
  if (pathname.includes("/mensa_history")) return "Mensa Historie";
  if (pathname.includes("/room_history")) return "Raum Historie";
  return "";
}

/**
 * Check page type from pathname
 */
export interface PageTypeInfo {
  isStudentDetailPage: boolean;
  isStudentHistoryPage: boolean;
  isRoomDetailPage: boolean;
  isActivityDetailPage: boolean;
  isDatabaseSubPage: boolean;
  isDatabaseDeepPage: boolean;
}

export function getPageTypeInfo(pathname: string): PageTypeInfo {
  const isStudentPath = pathname.startsWith("/students/");
  const isStudentDetailPage =
    isStudentPath &&
    pathname !== "/students" &&
    pathname !== "/students/search" &&
    !pathname.includes("/feedback_history") &&
    !pathname.includes("/mensa_history") &&
    !pathname.includes("/room_history");

  const isStudentHistoryPage =
    isStudentPath &&
    (pathname.includes("/feedback_history") ||
      pathname.includes("/mensa_history") ||
      pathname.includes("/room_history"));

  const isRoomDetailPage =
    pathname.startsWith("/rooms/") && pathname !== "/rooms";

  const isActivityDetailPage =
    pathname.startsWith("/activities/") && pathname !== "/activities";

  const isDatabaseSubPage =
    pathname.startsWith("/database/") && pathname !== "/database";

  const isDatabaseDeepPage = pathname.split("/").length > 3;

  return {
    isStudentDetailPage,
    isStudentHistoryPage,
    isRoomDetailPage,
    isActivityDetailPage,
    isDatabaseSubPage,
    isDatabaseDeepPage,
  };
}
