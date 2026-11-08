import "server-only";
import { env } from "~/env";
import {
  PORTAL_SESSION_COOKIE_NAMES,
  tenantSessionCookieName,
} from "~/lib/session-cookie-exit.server";
import type { LoginRefusalCode } from "~/lib/login-refusal";
import {
  PORTAL_ROUTES,
  portalSignInPage,
  type PortalKey,
} from "~/lib/portal-paths";
import {
  exactOriginRedirect,
  parentDomainRedirect,
  subdomainRedirect,
  type RedirectPolicy,
} from "./redirect-policies";

/**
 * Every portal's facts in one place (tenant, operator, parents, school,
 * caterer, Träger). The Auth.js configs, entry modules, logout and login
 * routes, layout snapshots, account-check exemptions and token validation
 * are derived from this table. The cross-stack contract (scopes, paths,
 * token kinds) is described in docs/agents/contracts.md.
 */

export type { PortalKey } from "~/lib/portal-paths";

/** The portal name POST /auth/session/validate expects. */
export type ValidationPortal =
  "tenant" | "platform" | "parent" | "school" | "caterer" | "org";

/** "operator": email is the username, the session carries role "operator". */
export type PortalIdentity = "account" | "operator";

/** A backend auth step a BFF route forwards; see `enveloped`. */
export type BackendAuthStep =
  | "login"
  | "mfa/verify"
  | "mfa/enroll/confirm"
  | "passkeys/login/verify"
  | "refresh";

export interface BackendEndpoint {
  path: string;
  /** The answer wraps its body in `{ data }`. */
  enveloped: boolean;
}

/** The token a backend logout authenticates and revokes the family of. */
export type LogoutTokenKind = "refresh" | "access";

export interface PasswordLogin {
  /** Backend codes and login steps the login page may read from the refusal;
   * any other refusal reads as identity.invalid_credentials. */
  refusals: readonly LoginRefusalCode[];
}

/** What a decoded session cookie must carry to hydrate a layout. */
export interface SnapshotClaims {
  tenantId?: unknown;
  orgId?: unknown;
}

export interface PortalDescriptor {
  key: PortalKey;
  validation: ValidationPortal;
  /** JWT scope claims a session of this portal may carry. */
  scopes: readonly (string | undefined)[];
  /** The scope stamped on new sessions; the tenant keeps the token's. */
  sessionScope?: string;
  identity: PortalIdentity;
  providerId: string;
  providerName: string;
  basePath: string;
  /** Auth.js' sign-in page; the App Router folder of the portal's login. */
  signInPage: string;
  /**
   * Null: Auth.js' default names (`authjs.*`, `__Secure-` on https), the
   * tenant on localhost.
   */
  cookieNames: {
    session: string;
    callbackUrl: string;
    csrf: string;
    sameSite: "lax" | "strict";
    domain?: string;
  } | null;
  redirect: RedirectPolicy;
  backend: {
    /** Prefix of the portal's own auth steps (`login`, `mfa/verify`, …). */
    authPrefix: string;
    refresh: BackendEndpoint;
    logout: { path: string; token: LogoutTokenKind };
    /** Steps whose answer is enveloped in `{ data }`. */
    enveloped: readonly BackendAuthStep[];
    /** The password step may ask which organization to sign in to. */
    organizationSelection?: true;
  };
  /**
   * A password sign-in through the Auth.js Credentials provider; only the
   * parents' login page uses one. Absent: sessions arrive only through
   * server-side token seeding (the BFF login routes) and, for the tenant,
   * the Träger hand-off.
   */
  passwordLogin?: PasswordLogin;
  snapshotClaims?: (claims: SnapshotClaims) => boolean;
  /** Discord, school slug login, Träger hand-off and the staff preview. */
  tenantFeatures?: true;
}

const ACCOUNT_REFRESH: BackendEndpoint = {
  path: "/auth/refresh",
  enveloped: false,
};
const ACCOUNT_LOGOUT = { path: "/auth/logout", token: "refresh" } as const;

/**
 * Host-only cookies (no domain) for every portal but the tenant: the browser
 * scopes them to the portal's host, so no other host ever sees them.
 */
function hostOnlyCookies(
  session: string,
  sameSite: "lax" | "strict",
): NonNullable<PortalDescriptor["cookieNames"]> {
  const prefix = session.slice(0, -".session-token".length);
  return {
    session,
    callbackUrl: `${prefix}.callback-url`,
    csrf: `${prefix}.csrf-token`,
    sameSite,
  };
}

/**
 * The tenant cookie spans every school subdomain for school switching. Its
 * names derive from TENANT_DOMAIN, so environments that share a parent
 * domain (staging.moto-app.de under moto-app.de) do not collide; colliding
 * names made Auth.js pick the wrong cookie (MissingCSRF). On localhost the
 * cookie stays host-only and switching needs a new login.
 */
function tenantCookies(): PortalDescriptor["cookieNames"] {
  if (!env.TENANT_DOMAIN || env.TENANT_DOMAIN === "localhost") return null;
  return {
    ...hostOnlyCookies(tenantSessionCookieName(env.TENANT_DOMAIN), "lax"),
    domain: `.${env.TENANT_DOMAIN}`,
  };
}

export const PORTALS = {
  tenant: {
    key: "tenant",
    validation: "tenant",
    // A tenant session's token carries no scope, "tenant", or "org" for an
    // Absprung out of the Träger portal.
    scopes: [undefined, "", "tenant", "org"],
    identity: "account",
    providerId: "credentials",
    providerName: "Credentials",
    basePath: PORTAL_ROUTES.tenant.basePath,
    signInPage: portalSignInPage("tenant"),
    cookieNames: tenantCookies(),
    redirect: parentDomainRedirect,
    backend: {
      authPrefix: "/auth",
      refresh: ACCOUNT_REFRESH,
      logout: ACCOUNT_LOGOUT,
      enveloped: [],
    },
    snapshotClaims: ({ tenantId }) => typeof tenantId === "number",
    tenantFeatures: true,
  },
  operator: {
    key: "operator",
    validation: "platform",
    scopes: ["platform"],
    sessionScope: "platform",
    identity: "operator",
    providerId: "operator-credentials",
    providerName: "Operator Credentials",
    basePath: PORTAL_ROUTES.operator.basePath,
    signInPage: portalSignInPage("operator"),
    cookieNames: hostOnlyCookies(PORTAL_SESSION_COOKIE_NAMES.operator, "lax"),
    redirect: subdomainRedirect,
    backend: {
      authPrefix: "/operator/auth",
      refresh: { path: "/operator/auth/refresh", enveloped: true },
      // The operator session chain authenticates the access token.
      logout: { path: "/operator/auth/logout", token: "access" },
      enveloped: ["login", "mfa/verify", "mfa/enroll/confirm", "refresh"],
    },
  },
  parent: {
    key: "parent",
    validation: "parent",
    scopes: ["parent"],
    sessionScope: "parent",
    identity: "account",
    providerId: "parent-credentials",
    providerName: "Parent Credentials",
    basePath: PORTAL_ROUTES.parent.basePath,
    signInPage: portalSignInPage("parent"),
    // Strict: the portal handles guardian PII and has no cross-site flow
    // that needs the cookie; login is POST-only.
    cookieNames: hostOnlyCookies(PORTAL_SESSION_COOKIE_NAMES.parent, "strict"),
    redirect: exactOriginRedirect,
    backend: {
      authPrefix: "/parent/auth",
      refresh: ACCOUNT_REFRESH,
      logout: ACCOUNT_LOGOUT,
      enveloped: ["login"],
    },
    passwordLogin: {
      // care.not_a_guardian is reachable only after the password check
      // (LoginParentWithAudit), so naming it confirms nothing to a stranger.
      refusals: [
        "identity.session_account_inactive",
        "care.not_a_guardian",
        // None of the guardian's schools booked Eltern-App (#4895).
        "module.not_booked",
      ],
    },
  },
  school: {
    key: "school",
    validation: "school",
    scopes: ["school"],
    sessionScope: "school",
    identity: "account",
    providerId: "school-credentials",
    providerName: "School Credentials",
    basePath: PORTAL_ROUTES.school.basePath,
    signInPage: portalSignInPage("school"),
    cookieNames: hostOnlyCookies(PORTAL_SESSION_COOKIE_NAMES.school, "strict"),
    redirect: exactOriginRedirect,
    backend: {
      authPrefix: "/school/auth",
      refresh: ACCOUNT_REFRESH,
      logout: ACCOUNT_LOGOUT,
      enveloped: [],
    },
    // School tokens are tenant-bound; the portal keys its data cache on it.
    snapshotClaims: ({ tenantId }) =>
      typeof tenantId === "number" && tenantId > 0,
  },
  caterer: {
    key: "caterer",
    validation: "caterer",
    scopes: ["caterer"],
    sessionScope: "caterer",
    identity: "account",
    providerId: "caterer-credentials",
    providerName: "Caterer Credentials",
    basePath: PORTAL_ROUTES.caterer.basePath,
    signInPage: portalSignInPage("caterer"),
    cookieNames: hostOnlyCookies(PORTAL_SESSION_COOKIE_NAMES.caterer, "strict"),
    redirect: exactOriginRedirect,
    backend: {
      authPrefix: "/caterer/auth",
      refresh: ACCOUNT_REFRESH,
      logout: ACCOUNT_LOGOUT,
      enveloped: ["login"],
    },
  },
  traeger: {
    key: "traeger",
    validation: "org",
    scopes: ["org"],
    sessionScope: "org",
    identity: "account",
    providerId: "traeger-credentials",
    providerName: "Träger Credentials",
    basePath: PORTAL_ROUTES.traeger.basePath,
    signInPage: portalSignInPage("traeger"),
    cookieNames: hostOnlyCookies(PORTAL_SESSION_COOKIE_NAMES.traeger, "strict"),
    redirect: exactOriginRedirect,
    backend: {
      authPrefix: "/traeger/auth",
      refresh: ACCOUNT_REFRESH,
      logout: ACCOUNT_LOGOUT,
      enveloped: [],
      organizationSelection: true,
    },
    // A Träger session never names a school (ADR 0084): a school session of
    // the Absprung lives in the tenant cookie.
    snapshotClaims: ({ tenantId, orgId }) =>
      typeof orgId === "number" && orgId > 0 && tenantId == null,
  },
} as const satisfies Record<PortalKey, PortalDescriptor>;

/** Every portal, for code that derives one entry per portal. */
export const ALL_PORTALS: readonly PortalDescriptor[] = Object.values(PORTALS);

/** The portal a session's scope claim belongs to; the tenant by default. */
export function portalOfScope(scope: unknown): PortalDescriptor {
  return (
    ALL_PORTALS.find(
      (portal) =>
        portal.key !== "tenant" && portal.scopes.includes(scope as string),
    ) ?? PORTALS.tenant
  );
}

export function acceptsScope(
  portal: ValidationPortal,
  scope: string | undefined,
): boolean {
  return ALL_PORTALS.find((p) => p.validation === portal)!.scopes.includes(
    scope,
  );
}
