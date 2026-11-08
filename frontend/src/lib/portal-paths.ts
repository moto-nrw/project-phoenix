/**
 * The browser-safe part of the portal table (`server/auth/portals.ts`): each
 * portal's key and the base path of its BFF auth routes. The server table
 * takes its keys and base paths from here, so browser code can name a
 * portal's routes without importing server code (#5005).
 */
export const PORTAL_AUTH_BASE_PATHS = {
  tenant: "/api/auth",
  operator: "/api/operator/auth",
  parent: "/api/parent/auth",
  school: "/api/school/auth",
  caterer: "/api/caterer/auth",
  traeger: "/api/traeger/auth",
} as const;

export type PortalKey = keyof typeof PORTAL_AUTH_BASE_PATHS;

/** A BFF auth route of a portal, such as `logout` or `mfa/verify`. */
export function portalAuthPath(portal: PortalKey, step: string): string {
  return `${PORTAL_AUTH_BASE_PATHS[portal]}/${step}`;
}
