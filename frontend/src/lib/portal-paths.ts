/**
 * The browser- and edge-safe part of the portal table
 * (`server/auth/portals.ts`): each portal's key, the base path of its BFF
 * auth routes and, for a portal on its own host, the host's env name and the
 * App Router folder of its pages. The server table, the proxy, the cookie
 * lock and browser code derive their portal facts from here, so none of
 * them imports server code (#5005).
 */

/**
 * An auth step whose answer a portal may wrap in `{ status, data }`: the
 * backend's answer, which the BFF route passes on in the same shape.
 */
export type EnvelopableAuthStep =
  | "login"
  | "mfa/verify"
  | "mfa/enroll/confirm"
  | "mfa/resend"
  | "mfa/trusted-devices"
  | "passkeys/login/verify"
  | "refresh";

interface PortalRoutes {
  readonly basePath: `/api/${string}`;
  /** The client log route; it authenticates with this portal's session. */
  readonly logPath: `/api/${string}`;
  /** The Auth.js Credentials provider id, for `signIn` in the browser. */
  readonly providerId: string;
  /** The steps whose answer is enveloped in `{ data }`, for server and browser. */
  readonly enveloped: readonly EnvelopableAuthStep[];
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
    logPath: "/api/operator/logs",
    enveloped: [
      "login",
      "mfa/verify",
      "mfa/enroll/confirm",
      "mfa/resend",
      "mfa/trusted-devices",
      "refresh",
    ],
    providerId: "operator-credentials",
    hostEnv: "NEXT_PUBLIC_OPERATOR_HOSTNAME",
    hostname: () => process.env.NEXT_PUBLIC_OPERATOR_HOSTNAME,
    appPrefix: "/operator",
  },
  parent: {
    basePath: "/api/parent/auth",
    logPath: "/api/parent/logs",
    enveloped: ["login"],
    providerId: "parent-credentials",
    hostEnv: "NEXT_PUBLIC_PARENTS_HOSTNAME",
    hostname: () => process.env.NEXT_PUBLIC_PARENTS_HOSTNAME,
    appPrefix: "/parents",
  },
  school: {
    basePath: "/api/school/auth",
    logPath: "/api/school/logs",
    enveloped: [],
    providerId: "school-credentials",
    hostEnv: "NEXT_PUBLIC_SCHOOL_HOSTNAME",
    hostname: () => process.env.NEXT_PUBLIC_SCHOOL_HOSTNAME,
    appPrefix: "/school",
  },
  caterer: {
    basePath: "/api/caterer/auth",
    logPath: "/api/caterer/logs",
    enveloped: ["login"],
    providerId: "caterer-credentials",
    hostEnv: "NEXT_PUBLIC_CATERER_HOSTNAME",
    hostname: () => process.env.NEXT_PUBLIC_CATERER_HOSTNAME,
    appPrefix: "/caterer",
  },
  traeger: {
    basePath: "/api/traeger/auth",
    logPath: "/api/traeger/logs",
    enveloped: [],
    providerId: "traeger-credentials",
    hostEnv: "NEXT_PUBLIC_TRAEGER_HOSTNAME",
    hostname: () => process.env.NEXT_PUBLIC_TRAEGER_HOSTNAME,
    appPrefix: "/traeger",
  },
} as const satisfies Record<string, HostedPortalRoutes>;

export const PORTAL_ROUTES = {
  tenant: {
    basePath: "/api/auth",
    logPath: "/api/logs",
    providerId: "credentials",
    enveloped: [],
  } satisfies PortalRoutes,
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

export function portalLogPath(portal: PortalKey): string {
  return PORTAL_ROUTES[portal].logPath;
}

function envelopes(portal: PortalKey, step: EnvelopableAuthStep): boolean {
  const steps: readonly EnvelopableAuthStep[] = PORTAL_ROUTES[portal].enveloped;
  return steps.includes(step);
}

/** The body of a portal's answer to an auth step, out of its envelope. */
export function portalAuthAnswer<T>(
  portal: PortalKey,
  step: EnvelopableAuthStep,
  answer: unknown,
): T {
  return (envelopes(portal, step) ? (answer as { data: T }).data : answer) as T;
}

/** The answer in the shape the portal's auth step uses. */
export function portalAuthEnvelope(
  portal: PortalKey,
  step: EnvelopableAuthStep,
  body: Record<string, unknown>,
): Record<string, unknown> {
  return envelopes(portal, step) ? { status: "success", data: body } : body;
}

/** The page Auth.js sends a signed-out visitor to; the proxy rewrites
 * `/login` on the portal's host to it. */
export function portalSignInPage(portal: PortalKey): string {
  return portal === "tenant" ? "/" : `${PORTAL_ROUTES[portal].appPrefix}/login`;
}

/**
 * A hosted portal's login page as the browser reaches it: `/login` on the
 * portal's own host, the prefixed page elsewhere (the proxy redirects it).
 */
export function portalLoginPath(portal: HostedPortalKey): string {
  const onOwnHost =
    typeof window === "undefined" ||
    window.location.host === PORTAL_ROUTES[portal].hostname();
  return onOwnHost ? "/login" : portalSignInPage(portal);
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

/**
 * The portal of a request host on the server, where a decision depends on it
 * (which session to read or mint): like `portalOfHost`, but an unset portal
 * host throws instead of letting that host read as a school's. Compares host
 * names; ports do not separate cookies either.
 */
export function configuredPortalOfHost(authority: string): PortalKey {
  const hostname = hostnameOf(authority);
  return (
    HOSTED_PORTAL_KEYS.find(
      (key) => hostnameOf(requiredPortalHostname(key)) === hostname,
    ) ?? "tenant"
  );
}

function hostnameOf(authority: string): string {
  return new URL(`http://${authority}`).hostname;
}

/** A portal's host name; throws naming the env variable when it is unset. */
export function requiredPortalHostname(portal: HostedPortalKey): string {
  const hostname = PORTAL_ROUTES[portal].hostname();
  if (!hostname) throw new Error(`${PORTAL_ROUTES[portal].hostEnv} is not set`);
  return hostname;
}
