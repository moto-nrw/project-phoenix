import type { NextRequest } from "next/server";
import { auth, setTenantSession } from "~/server/auth";
import { withTenantAuth } from "~/server/auth/tenant-route";
import { sessionMutationOriginError } from "~/server/auth/same-origin";
import { proxyJsonRequest } from "~/lib/backend-proxy-route.server";
import { getServerApiUrl } from "~/lib/server-api-url";
import { getClientForwardHeaders } from "~/lib/client-headers.server";
import { problemResponse } from "~/lib/api-response-error.server";
import { createLogger } from "~/lib/logger";
import type { RouteContext } from "~/lib/route-wrapper-utils.server";

const logger = createLogger({ component: "TenantSwitch" });

export const POST = withTenantAuth(
  async (request: NextRequest, context: RouteContext) => {
    const originError = sessionMutationOriginError(request);
    if (originError) return originError;
    const session = await auth();
    if (!session?.user?.token)
      return problemResponse(401, "Unauthorized", request);
    return proxyJsonRequest(
      request,
      context,
      {
        method: "POST",
        path: "/auth/switch-tenant",
        onSuccess: async (response) => {
          const pair = (await response.json().catch(() => {
            throw new Error("Invalid session transition response");
          })) as {
            access_token: string;
            refresh_token: string;
          };
          try {
            await setTenantSession(pair.access_token, pair.refresh_token);
          } catch {
            // The new family must not survive a failed cookie handoff. Do not
            // return its credential, including if revocation is unavailable.
            try {
              const rollback = await fetch(`${getServerApiUrl()}/auth/logout`, {
                method: "POST",
                headers: {
                  ...getClientForwardHeaders(request),
                  Authorization: `Bearer ${pair.refresh_token}`,
                },
                signal: AbortSignal.timeout(5000),
              });
              if (!rollback.ok)
                logger.error("tenant_switch_rollback_failed", {
                  status: rollback.status,
                });
            } catch {
              logger.error("tenant_switch_rollback_failed", {});
            }
            return problemResponse(500, "Session switch failed", request);
          }
          return Response.json({ success: true });
        },
      },
      session.user.token,
    );
  },
);
