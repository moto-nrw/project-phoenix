import "server-only";
import type { NextRequest } from "next/server";
import { env } from "~/env";
import { problemResponse } from "~/lib/api-response-error.server";
import {
  clientIdentityHeaders,
  getOriginalRequestHost,
  hostnameFromAuthority,
} from "~/lib/client-headers.server";
import { createLogger } from "~/lib/logger";
import { RESERVED_SLUGS } from "~/lib/reserved-slugs";
import { lookupTenant, TENANT_RATE_LIMITED } from "~/lib/tenant-resolve.server";
import { isValidTenantSlug } from "~/lib/tenant-slug";
import { TENANT_SESSION_MISMATCH } from "~/lib/tenant-session-mismatch";
import type { ServerSession } from "./shared";

const logger = createLogger({ component: "TenantHostSession" });

/**
 * Routes that act on the session itself, not on a school's data. TenantGuard
 * and the login recovery call them exactly while the session belongs to
 * another school than the host: switching to the host's school, ending a
 * staff preview before that switch, renewing the token for that switch,
 * signing out, and its own log lines.
 */
const SESSION_ROUTES = new Set([
  "/api/auth/switch-tenant",
  "/api/auth/staff-preview/end",
  "/api/auth/token",
  "/api/auth/logout",
  "/api/logs",
]);

/** The school subdomain of a tenant host, or null for any other host. */
export function tenantSubdomainOfHost(authority: string): string | null {
  const hostname = hostnameFromAuthority(authority);
  const domain = env.TENANT_DOMAIN;
  if (!hostname || !hostname.endsWith(`.${domain}`)) return null;
  const subdomain = hostname.slice(0, -(domain.length + 1));
  if (!isValidTenantSlug(subdomain) || RESERVED_SLUGS.has(subdomain)) {
    return null;
  }
  return subdomain;
}

/**
 * The tenant session cookie is shared by all school subdomains. A switch in
 * another tab replaces it, and the backend only knows the school of the
 * token. So a tenant route answers a session of another school than its host
 * itself, before any request reaches the backend (#5130). Without a session,
 * without a school in it, or on a host without a school, the route decides as
 * before.
 */
export async function rejectForeignTenantSession(
  request: NextRequest,
  session: ServerSession | null,
): Promise<Response | null> {
  const sessionTenantId = session?.user?.tenantId;
  if (sessionTenantId === undefined) return null;
  if (SESSION_ROUTES.has(request.nextUrl.pathname)) return null;

  const subdomain = tenantSubdomainOfHost(getOriginalRequestHost(request));
  if (!subdomain) return null;

  const hostTenant = await lookupTenant(
    subdomain,
    clientIdentityHeaders(request.headers),
  ).catch((error: unknown) => {
    logger.warn("tenant_host_lookup_failed", {
      subdomain,
      error: error instanceof Error ? error.message : String(error),
    });
    return undefined;
  });
  // Without the host's school the request cannot be checked; it must not
  // reach the backend unchecked.
  if (hostTenant === undefined || hostTenant === TENANT_RATE_LIMITED) {
    return problemResponse(
      503,
      "tenant of the request host could not be resolved",
      { code: "general.unavailable" },
      request,
    );
  }
  // A subdomain without a school (null) matches no session.
  if (hostTenant?.tenant_id === sessionTenantId) return null;

  logger.warn("tenant_session_host_mismatch", {
    session_tenant_id: sessionTenantId,
    host_tenant_id: hostTenant?.tenant_id,
    subdomain,
    path: request.nextUrl.pathname,
  });
  return problemResponse(
    409,
    "session belongs to another school than the request host",
    { code: TENANT_SESSION_MISMATCH },
    request,
  );
}
