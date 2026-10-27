import "server-only";
import type { NextRequest } from "next/server";
import { clientIdentityHeaders } from "~/lib/client-headers.server";
import { problemResponse } from "~/lib/api-response-error.server";
import { createLogger } from "~/lib/logger";
import { isBareTenantDomainHost, tenantSlugFromHost } from "~/lib/tenant-host";
import {
  resolveTenantCached,
  TenantResolveStatusError,
} from "~/lib/tenant-resolve.server";
import type { ServerSession } from "./shared";

const logger = createLogger({ component: "TenantHostCheck" });

/**
 * Routes that must work while the cookie belongs to another school: the
 * switch back to this host's school (TenantGuard), the token refresh, the
 * logout, the audit call that ends a staff preview before the switch, and
 * the tab's own log lines about all of that. Accepting an invitation with an
 * existing account acts for the school the invitation names, not for the
 * session's; the bare domain's invitation page calls it.
 */
const MISMATCH_EXEMPT_PATHS = new Set([
  "/api/auth/switch-tenant",
  "/api/auth/token",
  "/api/auth/logout",
  "/api/auth/staff-preview/end",
  "/api/logs",
  "/api/invitations/accept",
]);

function mismatchResponse(
  request: NextRequest,
  context: Record<string, unknown>,
): Response {
  logger.warn("session_tenant_mismatch", context);
  return problemResponse(
    409,
    "The session belongs to another school than this host",
    { code: "identity.session_tenant_mismatch" },
    request,
  );
}

/**
 * Refuses a request whose session belongs to another school than the
 * subdomain it was sent to (#5130). The tenant cookie is shared across
 * school subdomains: a school switch in a tab of school B replaces it, and
 * tab A on school A cannot hear of it. Without this check the BFF answered
 * tab A with school B's data and ran its actions in school B; RLS limits a
 * request to the school of its token, not to the one of its host.
 *
 * Returns null when the request may pass: no tenant session, a host of
 * another portal, an exempt route, or the same school. The bare domain
 * refuses a school's session. The host's school comes from the cached tenant lookup the tenant
 * layout uses. A failed lookup refuses the request instead of letting it
 * through unchecked.
 */
export async function sessionTenantMismatchResponse(
  request: NextRequest,
  session: ServerSession | null,
): Promise<Response | null> {
  const sessionTenantId = session?.user?.tenantId;
  if (typeof sessionTenantId !== "number") return null;
  if (MISMATCH_EXEMPT_PATHS.has(request.nextUrl.pathname)) return null;

  const host = request.headers.get("host") ?? request.nextUrl.host;
  const subdomain = tenantSlugFromHost(host);
  if (!subdomain) {
    // The bare domain shows no school since the tenant layout sends path
    // mode there to the school's subdomain. A school's session from there
    // comes from a tab still on path mode; nothing binds it to that tab's
    // school, so its tenant requests stop.
    return isBareTenantDomainHost(host)
      ? mismatchResponse(request, {
          session_tenant_id: sessionTenantId,
          host: "bare_domain",
        })
      : null;
  }

  let hostTenantId: number;
  try {
    hostTenantId = (
      await resolveTenantCached(
        subdomain,
        clientIdentityHeaders(request.headers),
      )
    ).tenant_id;
  } catch (error) {
    const status =
      error instanceof TenantResolveStatusError ? error.status : undefined;
    logger.warn("tenant_host_resolve_failed", {
      status,
      error: error instanceof Error ? error.message : String(error),
    });
    return problemResponse(
      status === 429 ? 429 : 503,
      "The school of this host could not be resolved",
      request,
    );
  }

  if (hostTenantId === sessionTenantId) return null;

  return mismatchResponse(request, {
    session_tenant_id: sessionTenantId,
    host_tenant_id: hostTenantId,
  });
}
