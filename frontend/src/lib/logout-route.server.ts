import "server-only";
import type { NextRequest } from "next/server";
import { forwardBackendResponse } from "~/lib/backend-proxy-response.server";
import {
  handleApiError,
  problemResponse,
} from "~/lib/api-response-error.server";
import { sendBackend } from "~/lib/backend-client.server";
import { createLogger } from "~/lib/logger";
import type { PortalAuth } from "~/server/auth/portal-auth";
import type { PortalDescriptor } from "~/server/auth/portals";
import type { ServerSession } from "~/server/auth/session-types";

const logger = createLogger({ component: "PortalLogoutRoute" });

function logoutToken(
  portal: PortalDescriptor,
  session: ServerSession | null,
): string | undefined {
  return portal.backend.logout.token === "access"
    ? session?.user?.token
    : session?.user?.refreshToken;
}

/**
 * A portal's BFF logout (#5194): revokes the session's refresh-token family,
 * so its access tokens end at once, also in a cookie a late response writes
 * back. The browser takes 401 for "nothing left to revoke" and then signs
 * out. A refused token may still belong to a live family (rotated by another
 * tab, or an expired access token), so the session is renewed once and the
 * logout repeated with its token: a revoked family ends in the renewal and
 * its 401 stands; a renewal that yields no new token answers 503, and the
 * browser keeps the session and offers the logout again.
 */
export function createLogoutRoute(
  session: Pick<PortalAuth, "portal" | "auth" | "uncachedAuth" | "withAuth">,
) {
  const { portal, auth, uncachedAuth } = session;
  const revoke = (request: NextRequest, token: string) =>
    sendBackend(portal.backend.logout.path, token, {
      method: "POST",
      request,
      timeout: "logout",
    });

  return session.withAuth(async (request: NextRequest) => {
    try {
      const token = logoutToken(portal, await auth());
      if (!token) return problemResponse(401, "No active session", request);

      const response = await revoke(request, token);
      if (response.status !== 401) {
        return forwardBackendResponse(response, request);
      }
      const renewed = logoutToken(portal, await uncachedAuth());
      if (!renewed) return forwardBackendResponse(response, request);
      if (renewed === token) {
        return problemResponse(
          503,
          "Logout could not confirm the revocation",
          request,
        );
      }
      return forwardBackendResponse(await revoke(request, renewed), request);
    } catch (error) {
      logger.error("logout failed", {
        portal: portal.key,
        error: error instanceof Error ? error.message : String(error),
      });
      return handleApiError(error, request);
    }
  });
}
