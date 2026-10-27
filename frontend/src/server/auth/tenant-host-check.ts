import "server-only";
import type { NextRequest } from "next/server";
import { clientIdentityHeaders } from "~/lib/client-headers.server";
import { problemResponse } from "~/lib/api-response-error.server";
import { createLogger } from "~/lib/logger";
import { tenantSlugFromHost } from "~/lib/tenant-host";
import {
  resolveTenantCached,
  TenantResolveStatusError,
} from "~/lib/tenant-resolve.server";
import type { ServerSession } from "./shared";

const logger = createLogger({ component: "TenantHostCheck" });

/**
 * Routes that must work while the cookie belongs to another school: the
 * switch back to this host's school (TenantGuard), the token refresh, the
 * logout and the audit call that ends a staff preview before the switch.
 */
const MISMATCH_EXEMPT_PATHS = new Set([
  "/api/auth/switch-tenant",
  "/api/auth/token",
  "/api/auth/logout",
  "/api/auth/staff-preview/end",
]);

/**
 * Refuses a request whose session belongs to another school than the
 * subdomain it was sent to (#5130). The tenant cookie is shared across
 * school subdomains: a school switch in a tab of school B replaces it, and
 * tab A on school A cannot hear of it. Without this check the BFF answered
 * tab A with school B's data and ran its actions in school B; RLS limits a
 * request to the school of its token, not to the one of its host.
 *
 * Returns null when the request may pass: no tenant session, a host without
 * a school subdomain (bare domain, path mode), an exempt route, or the same
 * school. The host's school comes from the cached tenant lookup the tenant
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

  const subdomain = tenantSlugFromHost(
    request.headers.get("host") ?? request.nextUrl.host,
  );
  if (!subdomain) return null;

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

  logger.warn("session_tenant_mismatch", {
    session_tenant_id: sessionTenantId,
    host_tenant_id: hostTenantId,
  });
  return problemResponse(
    409,
    "The session belongs to another school than this host",
    { code: "identity.session_tenant_mismatch" },
    request,
  );
}
