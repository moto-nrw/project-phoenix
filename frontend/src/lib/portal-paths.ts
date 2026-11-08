/**
 * The browser- and edge-safe part of the portal table
 * (`server/auth/portals.ts`): each portal's key, the base path of its BFF
 * auth routes and, for a portal on its own host, the host's env name and the
 * App Router folder of its pages. The server table, the proxy, the cookie
 * lock and browser code derive their portal facts from here, so none of
 * them imports server code (#5005).
 */

interface PortalRoutes {
  readonly basePath: `/api/${string}`;
}

interface HostedPortalRoutes extends PortalRoutes {
  /** The env name of the portal's host, for the proxy's fail-fast check. */
  readonly hostEnv: string;
  /**
   * The host name. Static `process.env.X` access: Next.js inlines
   * NEXT_PUBLIC_* values only in that form.
   */
  readonly hostname: () => string | undefined;
  /** Path namespace of the portal's pages inside the App Router. */
  readonly appPrefix: `/${string}`;
}

const HOSTED_PORTAL_ROUTES = {
  operator: {
    basePath: "/api/operator/auth",
    hostEnv: "NEXT_PUBLIC_OPERATOR_HOSTNAME",
    hostname: () => process.env.NEXT_PUBLIC_OPERATOR_HOSTNAME,
    appPrefix: "/operator",
  },
  parent: {
    basePath: "/api/parent/auth",
    hostEnv: "NEXT_PUBLIC_PARENTS_HOSTNAME",
    hostname: () => process.env.NEXT_PUBLIC_PARENTS_HOSTNAME,
    appPrefix: "/parents",
  },
  school: {
    basePath: "/api/school/auth",
    hostEnv: "NEXT_PUBLIC_SCHOOL_HOSTNAME",
    hostname: () => process.env.NEXT_PUBLIC_SCHOOL_HOSTNAME,
    appPrefix: "/school",
  },
  caterer: {
    basePath: "/api/caterer/auth",
    hostEnv: "NEXT_PUBLIC_CATERER_HOSTNAME",
    hostname: () => process.env.NEXT_PUBLIC_CATERER_HOSTNAME,
    appPrefix: "/caterer",
  },
  traeger: {
    basePath: "/api/traeger/auth",
    hostEnv: "NEXT_PUBLIC_TRAEGER_HOSTNAME",
    hostname: () => process.env.NEXT_PUBLIC_TRAEGER_HOSTNAME,
    appPrefix: "/traeger",
  },
} as const satisfies Record<string, HostedPortalRoutes>;

export const PORTAL_ROUTES = {
  tenant: { basePath: "/api/auth" } satisfies PortalRoutes,
  ...HOSTED_PORTAL_ROUTES,
};

export type PortalKey = keyof typeof PORTAL_ROUTES;

/** A portal on its own host: every portal but the tenant. */
export type HostedPortalKey = keyof typeof HOSTED_PORTAL_ROUTES;

export const PORTAL_KEYS = Object.keys(PORTAL_ROUTES) as PortalKey[];

export const HOSTED_PORTAL_KEYS = Object.keys(
  HOSTED_PORTAL_ROUTES,
) as HostedPortalKey[];

/** A BFF auth route of a portal, such as `logout` or `mfa/verify`. */
export function portalAuthPath(portal: PortalKey, step: string): string {
  return `${PORTAL_ROUTES[portal].basePath}/${step}`;
}

/**
 * A portal's client log route, beside its auth routes: `/api/logs` for the
 * tenant, `/api/<portal>/logs` for the others. It authenticates with that
 * portal's session, so the tenant cookie is never needed for it.
 */
export function portalLogPath(portal: PortalKey): string {
  return PORTAL_ROUTES[portal].basePath.replace(/\/auth$/, "/logs");
}

/** The page Auth.js sends a signed-out visitor to; the proxy rewrites
 * `/login` on the portal's host to it. */
export function portalSignInPage(portal: PortalKey): string {
  return portal === "tenant" ? "/" : `${PORTAL_ROUTES[portal].appPrefix}/login`;
}

/**
 * The portal of a browser host: a portal host or, for every other host, the
 * tenant. Does not fail on a missing host name, so callers that must not
 * throw (the logger) can use it.
 */
export function portalOfHost(host: string): PortalKey {
  return (
    HOSTED_PORTAL_KEYS.find((key) => PORTAL_ROUTES[key].hostname() === host) ??
    "tenant"
  );
}
