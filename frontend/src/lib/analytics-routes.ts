/**
 * Explicit allowlist of authenticated tenant pages that may be sent to
 * product analytics. Dynamic URL segments are represented by their route
 * parameter name, never by the value from the browser URL.
 */
export const TRACKED_TENANT_ROUTE_TEMPLATES = [
  "/active-supervisions",
  "/activities",
  "/admin/change-requests",
  "/admin/enrollments",
  "/admin/enrollments/:id",
  "/admin/enrollments/change-requests",
  "/admin/enrollments/change-requests/:id",
  "/admin/enrollments/phases/:phaseId",
  "/admin/guardian-approvals",
  "/betreuungsplan",
  "/calendar",
  "/calendar-periods",
  "/care-offerings",
  "/dashboard",
  "/database",
  "/database/activities",
  "/database/devices",
  "/database/exports",
  "/database/grade-transitions",
  "/database/groups",
  "/database/permissions",
  "/database/personal",
  "/database/personal/import",
  "/database/personal/opening-balances",
  "/database/roles",
  "/database/rooms",
  "/database/students",
  "/database/students/import",
  "/day-log",
  "/dienstplan",
  "/eltern",
  "/emergency",
  "/enrollment-form",
  "/enrollment-phases",
  "/enrollment-phases/:id/review",
  "/info-displays",
  "/invitations",
  "/klassen",
  "/lists",
  "/meal-plan",
  "/messages",
  "/messages/:threadId",
  "/ogs-groups",
  "/parent-announcements",
  "/payroll",
  "/planung",
  "/profile",
  "/reminders",
  "/rooms",
  "/rooms/:id",
  "/settings",
  "/staff",
  "/staff/:id",
  "/staff/dienstplan",
  "/students/:id",
  "/students/:id/change-history",
  "/students/:id/feedback-history",
  "/students/:id/room-history",
  "/students/search",
  "/substitutions",
  "/suggestions",
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
