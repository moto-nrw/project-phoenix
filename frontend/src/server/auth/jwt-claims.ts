import "server-only";
import type { JWT } from "next-auth/jwt";
import { createLogger } from "~/lib/logger";
import type { JwtPayload } from "./jwt-payload";
import type { PortalIdentity } from "./portals";
import type { AuthUser } from "./session-types";

export const authLogger = createLogger({ component: "NextAuthConfig" });

export function parseJwtPayload(tokenString: string): JwtPayload | null {
  const payloadPart = tokenString.split(".")[1];
  if (tokenString.split(".").length !== 3 || !payloadPart) {
    authLogger.error("invalid jwt token format", {});
    return null;
  }
  try {
    return JSON.parse(
      Buffer.from(payloadPart, "base64").toString(),
    ) as JwtPayload;
  } catch (e) {
    authLogger.error("error parsing jwt", {
      error: e instanceof Error ? e.message : String(e),
    });
    return null;
  }
}

export function buildDisplayName(
  payload: JwtPayload,
  fallbackEmail: string,
  ultimateFallback = "User",
): string {
  if (payload.first_name && payload.last_name) {
    return `${payload.first_name} ${payload.last_name}`;
  }
  if (payload.first_name) {
    return payload.first_name;
  }
  return payload.username ?? (fallbackEmail || ultimateFallback);
}

/** The email a session seeded from verified claims carries. */
export function seededEmail(
  payload: JwtPayload,
  identity: PortalIdentity,
): string {
  return identity === "operator"
    ? (payload.username ?? payload.email ?? "")
    : (payload.email ?? payload.sub ?? "");
}

/**
 * Keeps the session claims on the identity of its current access token:
 * after every rotation and on entering, renewing and leaving a staff preview.
 */
export function syncTokenFromPayload(
  token: JWT,
  payload: JwtPayload,
  identity: PortalIdentity,
): void {
  token.tenantId = payload.tenant_id;
  token.orgId = payload.org_id;
  token.viaTraeger = payload.via_traeger ?? false;
  token.roles = payload.roles ?? [];
  token.roleIsSystem = payload.role_is_system ?? [];
  token.permissions = payload.permissions ?? [];
  token.isAdmin = payload.is_admin ?? false;
  token.scope = payload.scope;
  token.name = buildDisplayName(payload, token.email ?? "");
  token.firstName = payload.first_name;
  // An operator's email is its username, which the platform UI can change;
  // account emails come from the sign-in and no rotation changes them.
  if (identity === "operator" && payload.username) {
    token.email = payload.username;
  }
}

const listOf = <T>(value: T[] | undefined): T[] =>
  Array.isArray(value) ? value : [];

export function buildAuthUser(
  payload: JwtPayload,
  token: string,
  refreshToken: string,
  email: string,
  portal: { identity: PortalIdentity; sessionScope?: string },
): AuthUser {
  const operator = portal.identity === "operator";
  return {
    id: String(payload.id),
    name: buildDisplayName(payload, email),
    email,
    token,
    refreshToken,
    roles: operator ? ["operator"] : listOf(payload.roles),
    roleIsSystem: operator ? [true] : listOf(payload.role_is_system),
    permissions: listOf(payload.permissions),
    firstName: payload.first_name,
    isAdmin: payload.is_admin ?? false,
    tenantId: payload.tenant_id,
    orgId: payload.org_id,
    scope: portal.sessionScope ?? payload.scope,
    viaTraeger: payload.via_traeger ?? false,
  };
}
