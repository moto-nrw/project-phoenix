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

/**
 * A portal's client log route, beside its auth routes: `/api/logs` for the
 * tenant, `/api/<portal>/logs` for the others. It authenticates with that
 * portal's session, so the tenant cookie is never needed for it.
 */
export function portalLogPath(portal: PortalKey): string {
  return PORTAL_AUTH_BASE_PATHS[portal].replace(/\/auth$/, "/logs");
}

/**
 * The portal of a browser host: a portal host or, for every other host, the
 * tenant. Reads the inlined NEXT_PUBLIC_* names without failing on a missing
 * one, so callers that must not throw (the logger) can use it.
 */
export function portalOfHost(host: string): PortalKey {
  const hosts: [PortalKey, string | undefined][] = [
    ["operator", process.env.NEXT_PUBLIC_OPERATOR_HOSTNAME],
    ["parent", process.env.NEXT_PUBLIC_PARENTS_HOSTNAME],
    ["school", process.env.NEXT_PUBLIC_SCHOOL_HOSTNAME],
    ["caterer", process.env.NEXT_PUBLIC_CATERER_HOSTNAME],
    ["traeger", process.env.NEXT_PUBLIC_TRAEGER_HOSTNAME],
  ];
  return hosts.find(([, hostname]) => hostname === host)?.[0] ?? "tenant";
}
