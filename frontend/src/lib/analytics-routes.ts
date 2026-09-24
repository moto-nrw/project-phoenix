/**
 * Route allowlists of the usage analytics (Nutzungsanalyse, #3601).
 *
 * Every page a portal can show has one template here. The analytics filter
 * (`analytics-policy.ts`) rewrites `$current_url`, `$pathname`, and
 * `$referrer` to the template, so a dynamic URL segment leaves the browser
 * only as its route parameter name, never as its value. A page without a
 * template is sent as `UNKNOWN_ANALYTICS_PATH`. `analytics-routes.test.ts`
 * fails when a `page.tsx` has no template; see
 * `.claude/rules/usage-analytics.md`.
 *
 * Templates are the paths the browser shows: the parents and school hosts
 * serve `/parents/*` and `/school/*` without that prefix.
 */

/** Authenticated OGS pages under `app/[tenant]/(protected)`. */
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

/** Tenant-host pages outside `(protected)`: login, enrollment, display. */
export const TRACKED_TENANT_PUBLIC_ROUTE_TEMPLATES = [
  "/",
  "/anmeldung",
  "/anmeldung/:phaseId",
  "/anmeldung/preview",
  "/anmeldung/status/:token",
  "/anmeldung/status/:token/adjust",
  "/anmeldung/status/:token/edit",
  "/anmeldung/submitted",
  "/demo",
  "/display",
  "/invite",
  "/reset-password",
] as const;

/** Parents portal pages under `app/parents`, as the parents host shows them. */
export const TRACKED_PARENT_ROUTE_TEMPLATES = [
  "/",
  "/accept-guardian-invite/:token",
  "/anmeldung",
  "/anmeldung/:tenantSlug/:phaseId",
  "/anmeldung/status/:token",
  "/anmeldung/status/:token/adjust",
  "/anmeldung/status/:token/edit",
  "/calendar",
  "/children",
  "/children/:id",
  "/demo",
  "/invite",
  "/login",
  "/meal-plan",
  "/messages",
  "/messages/:studentId",
  "/news",
  "/reset-password",
  "/settings",
] as const;

/** School portal pages under `app/school`, as the school host shows them. */
export const TRACKED_SCHOOL_ROUTE_TEMPLATES = [
  "/",
  "/aufsichten",
  "/einstellungen",
  "/invite",
  "/klasse",
  "/login",
  "/nachrichten",
  "/nachrichten/:threadID",
  "/reset-password",
  "/tagesinformationen",
] as const;

/** Public pages of the bare domain: school choice, start, demo, help. */
export const TRACKED_PUBLIC_ROUTE_TEMPLATES = [
  "/",
  "/demo",
  "/help",
  // Optional catch-all `help/[[...topic]]`; `/help` itself is listed above.
  "/help/:topic*",
  "/help/nfc/erste-schritte",
  "/invite",
  "/onboarding",
  "/reset-password",
  "/start",
] as const;

export type AnalyticsRouteSurface = "ogs" | "parents" | "school" | "public";

/** Path of every page without a template; never the raw browser path. */
export const UNKNOWN_ANALYTICS_PATH = "/unknown";

interface RouteMatcher {
  readonly template: string;
  readonly pattern: RegExp;
}

function segmentPattern(segment: string): string {
  if (segment.startsWith(":")) {
    return segment.endsWith("*") ? "[^/]+(?:/[^/]+)*" : "[^/]+";
  }
  return segment.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

// Static templates win over dynamic ones: /rooms/unterwegs before /rooms/:id.
function buildMatchers(templates: readonly string[]): RouteMatcher[] {
  return [...templates]
    .sort((left, right) => left.split(":").length - right.split(":").length)
    .map((template) => ({
      template,
      pattern: new RegExp(
        `^${template.split("/").map(segmentPattern).join("/")}$`,
      ),
    }));
}

const matchersBySurface: Readonly<
  Record<AnalyticsRouteSurface, readonly RouteMatcher[]>
> = {
  ogs: buildMatchers([
    ...TRACKED_TENANT_ROUTE_TEMPLATES,
    ...TRACKED_TENANT_PUBLIC_ROUTE_TEMPLATES,
  ]),
  parents: buildMatchers(TRACKED_PARENT_ROUTE_TEMPLATES),
  school: buildMatchers(TRACKED_SCHOOL_ROUTE_TEMPLATES),
  public: buildMatchers(TRACKED_PUBLIC_ROUTE_TEMPLATES),
};

function matchTemplate(
  surface: AnalyticsRouteSurface,
  pathname: string,
): string | null {
  return (
    matchersBySurface[surface].find(({ pattern }) => pattern.test(pathname))
      ?.template ?? null
  );
}

function withoutPrefix(pathname: string, prefix: string): string | null {
  if (pathname === prefix) return "/";
  return pathname.startsWith(`${prefix}/`)
    ? pathname.slice(prefix.length)
    : null;
}

function resolveOnSurface(
  surface: AnalyticsRouteSurface,
  pathname: string,
): string | null {
  switch (surface) {
    case "parents":
      return matchTemplate(
        "parents",
        withoutPrefix(pathname, "/parents") ?? pathname,
      );
    case "school":
      return matchTemplate(
        "school",
        withoutPrefix(pathname, "/school") ?? pathname,
      );
    case "ogs": {
      // Path routing (`/<slug>/dashboard`) puts the tenant slug first. A
      // single segment stays as it is, so an unknown page never counts as
      // the login page `/`.
      const slugless = /^\/[^/]+\/./.test(pathname)
        ? pathname.replace(/^\/[^/]+/, "")
        : null;
      return (
        matchTemplate("ogs", pathname) ??
        (slugless ? matchTemplate("ogs", slugless) : null)
      );
    }
    case "public": {
      if (withoutPrefix(pathname, "/operator")) return null;
      const parentsPath = withoutPrefix(pathname, "/parents");
      if (parentsPath) return matchTemplate("parents", parentsPath);
      const schoolPath = withoutPrefix(pathname, "/school");
      if (schoolPath) return matchTemplate("school", schoolPath);
      return (
        matchTemplate("public", pathname) ?? resolveOnSurface("ogs", pathname)
      );
    }
  }
}

/**
 * Converts a browser pathname into the route template of its surface.
 * Unknown pages, operator pages, and paths with a query or fragment return
 * null; the analytics filter sends them as `UNKNOWN_ANALYTICS_PATH`.
 */
export function resolveAnalyticsRoute(
  surface: AnalyticsRouteSurface,
  pathname: string,
): string | null {
  if (!pathname.startsWith("/")) return null;
  if (pathname.includes("?") || pathname.includes("#")) return null;

  const normalized =
    pathname.length > 1 ? pathname.replace(/\/+$/, "") || "/" : pathname;
  return resolveOnSurface(surface, normalized);
}
