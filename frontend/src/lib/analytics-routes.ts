/**
 * Explicit allowlist of authenticated tenant pages that may be sent to
 * product analytics. Dynamic URL segments are represented by their route
 * parameter name, never by the value from the browser URL.
 */
export const TRACKED_TENANT_ROUTE_TEMPLATES = [
  "/absences",
  "/active-supervisions",
  "/activities",
  // Nur noch ein Redirect-Frame auf /anfragen (#2429); bleibt gelistet, weil
  // die Allowlist jede page.tsx unter (protected) abdecken muss.
  "/admin/change-requests",
  "/admin/enrollments",
  "/admin/enrollments/:id",
  "/admin/enrollments/change-requests",
  "/admin/enrollments/change-requests/:id",
  "/admin/enrollments/phases/:phaseId",
  "/admin/guardian-approvals",
  "/anfragen",
  "/betreuungsplan",
  "/calendar",
  "/calendar-periods",
  "/care-offerings",
  "/dashboard",
  "/database",
  "/database/absence-types",
  "/database/activities",
  "/database/categories",
  "/database/devices",
  "/database/exports",
  "/database/grade-transitions",
  "/database/groups",
  "/database/permissions",
  "/database/personal",
  "/database/personal/import",
  "/database/personal/opening-balances",
  "/database/planning-tracks",
  "/database/roles",
  "/database/rooms",
  "/database/shift-types",
  "/database/students",
  "/database/students/class-list",
  "/database/students/class-list/import",
  "/database/students/ended-care",
  "/database/students/import",
  "/dateien",
  "/day-log",
  "/dienstplan",
  "/eltern/bankverbindungen",
  "/emergency",
  "/enrollment-form",
  "/enrollment-phases",
  "/enrollment-phases/:id/review",
  "/enrollment-phases/:id/rollover",
  "/home",
  "/info-displays",
  "/invitations",
  "/lists",
  "/meal-plan",
  "/messages",
  "/messages/:threadId",
  "/ogs-groups",
  "/parent-announcements",
  // Objektansicht einer Elternmitteilung (#3115): nur die Route, nie der
  // Titel — die Kennung bleibt als :id stehen.
  "/parent-announcements/:id",
  "/payroll",
  "/planung",
  "/profile",
  "/reminders",
  "/rooms",
  "/rooms/:id",
  // Kinder ohne Raumzuweisung (#3115): war das Panel „Unterwegs" auf /rooms.
  "/rooms/unterwegs",
  "/settings",
  "/staff",
  "/staff/:id",
  "/staff/dienstplan",
  "/statistics",
  "/students/:id",
  "/students/:id/change-history",
  "/students/:id/feedback-history",
  "/students/:id/room-history",
  "/students/search",
  "/substitutions",
  // OGS-interner Team-Chat (#2598). Nur die Route, nie ein Nachrichteninhalt
  // oder eine Konto-ID — der Thread-Parameter bleibt als :threadID stehen.
  "/team-chat",
  "/team-chat/:threadID",
  "/tagesinformationen",
  "/tagesplan",
  "/time-tracking",
  "/timetables",
  "/vertretung",
  "/vertretungsplan",
] as const;

export type AnalyticsViewId = (typeof TRACKED_TENANT_ROUTE_TEMPLATES)[number];

const routeMatchers = [...TRACKED_TENANT_ROUTE_TEMPLATES]
  .sort((left, right) => {
    const leftDynamicSegments = left.split(":").length - 1;
    const rightDynamicSegments = right.split(":").length - 1;
    return leftDynamicSegments - rightDynamicSegments;
  })
  .map((template) => ({
    template,
    pattern: new RegExp(
      `^${template
        .split("/")
        .map((segment) =>
          segment.startsWith(":")
            ? "[^/]+"
            : segment.replace(/[.*+?^${}()|[\]\\]/g, "\\$&"),
        )
        .join("/")}$`,
    ),
  }));

const trackedTemplateSet = new Set<string>(TRACKED_TENANT_ROUTE_TEMPLATES);

export function isAnalyticsViewId(value: unknown): value is AnalyticsViewId {
  return typeof value === "string" && trackedTemplateSet.has(value);
}

/**
 * Converts a browser pathname into an allowlisted route template. Unknown,
 * public, operator, and parent-portal paths deliberately return null.
 */
export function resolveAnalyticsViewId(
  pathname: string,
): AnalyticsViewId | null {
  if (pathname.includes("?") || pathname.includes("#")) return null;

  const normalized =
    pathname.length > 1 ? pathname.replace(/\/+$/, "") : pathname;
  return (
    routeMatchers.find(({ pattern }) => pattern.test(normalized))?.template ??
    null
  );
}
