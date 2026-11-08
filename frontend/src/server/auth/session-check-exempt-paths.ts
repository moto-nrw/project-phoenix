import "server-only";
import { portalAuthPath, portalLogPath } from "~/lib/portal-paths";
import { ALL_PORTALS } from "./portals";

/**
 * Routes that must work while the cookie belongs to another school: the
 * switch back to this host's school (TenantGuard), the token refresh, the
 * logout, the audit call that ends a staff preview before the switch, and
 * the tab's own log lines about all of that. Accepting an invitation with an
 * existing account acts for the school the invitation names, not for the
 * session's; the bare domain's invitation page calls it.
 */
export const MISMATCH_EXEMPT_PATHS: ReadonlySet<string> = new Set([
  ...["switch-tenant", "token", "logout", "staff-preview/end"].map((step) =>
    portalAuthPath("tenant", step),
  ),
  portalLogPath("tenant"),
  "/api/invitations/accept",
]);

/**
 * Account exemptions are deliberately independent of school exemptions.
 * Logout is an intentional exit from the browser's current cookie session,
 * even when another tab replaced it. Logs must still record reconciliation
 * failures, but do not read or mutate account data; that holds for every
 * portal's log route. Token renewal only
 * persists the current cookie and returns no token or account data; it must
 * remain reachable for the existing 401 recovery path. Tenant switching
 * and preview-end act on the cookie's account and must match the
 * tab; invitation acceptance does not use the response-aware auth wrapper.
 */
export const ACCOUNT_CHECK_EXEMPT_PATHS: ReadonlySet<string> = new Set([
  ...ALL_PORTALS.map((portal) => portalLogPath(portal.key)),
  portalAuthPath("tenant", "token"),
  // Each portal's BFF logout route, app/api/…/auth/logout/route.ts.
  ...ALL_PORTALS.map((portal) => portalAuthPath(portal.key, "logout")),
]);
