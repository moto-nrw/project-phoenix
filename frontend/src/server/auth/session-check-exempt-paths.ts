import "server-only";

/**
 * Routes that must work while the cookie belongs to another school: the
 * switch back to this host's school (TenantGuard), the token refresh, the
 * logout, the audit call that ends a staff preview before the switch, and
 * the tab's own log lines about all of that. Accepting an invitation with an
 * existing account acts for the school the invitation names, not for the
 * session's; the bare domain's invitation page calls it.
 */
export const MISMATCH_EXEMPT_PATHS: ReadonlySet<string> = new Set([
  "/api/auth/switch-tenant",
  "/api/auth/token",
  "/api/auth/logout",
  "/api/auth/staff-preview/end",
  "/api/logs",
  "/api/invitations/accept",
]);

/**
 * Routes that must work while the cookie holds another account than the
 * tab (#5183): the routes above and the logout of every other portal.
 */
export const ACCOUNT_CHECK_EXEMPT_PATHS: ReadonlySet<string> = new Set([
  ...MISMATCH_EXEMPT_PATHS,
  "/api/operator/auth/logout",
  "/api/parent/auth/logout",
  "/api/school/auth/logout",
  "/api/caterer/auth/logout",
  "/api/traeger/auth/logout",
]);
