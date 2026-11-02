import type { NextRequest } from "next/server";
import { auth, setStaffPreviewSession } from "~/server/auth";
import { withTenantAuth } from "~/server/auth/tenant-route";
import { sessionMutationOriginError } from "~/server/auth/same-origin";
import { proxyJsonRequest } from "~/lib/backend-proxy-route.server";
import { getServerApiUrl } from "~/lib/server-api-url";
import { getClientForwardHeaders } from "~/lib/client-headers.server";
import { problemResponse } from "~/lib/api-response-error.server";
import { createLogger } from "~/lib/logger";
import type { RouteContext } from "~/lib/route-wrapper-utils.server";

const logger = createLogger({ component: "StaffPreviewStart" });

export const POST = withTenantAuth(
  async (request: NextRequest, context: RouteContext) => {
    const originError = sessionMutationOriginError(request);
    if (originError) return originError;
    const session = await auth();
    if (!session?.user?.token)
      return problemResponse(401, "Unauthorized", request);
    if (session.user.isPreview)
      return problemResponse(409, "A preview is already running", request);
    return proxyJsonRequest(
      request,
      context,
      {
        method: "POST",
        path: "/auth/staff-preview",
        forwardClientHeaders: true,
        onSuccess: async (response) => {
          const preview = (await response.json().catch(() => {
            throw new Error("Invalid session transition response");
          })) as {
            access_token: string;
            expires_in: number;
            target_account_id: string;
            target_name: string;
          };
          try {
            await setStaffPreviewSession({
              accessToken: preview.access_token,
              expiresIn: preview.expires_in,
              targetAccountId: preview.target_account_id,
              targetName: preview.target_name,
            });
          } catch {
            try {
              const rollback = await fetch(
                `${getServerApiUrl()}/auth/staff-preview/end`,
                {
                  method: "POST",
                  headers: {
                    ...getClientForwardHeaders(request),
                    "Content-Type": "application/json",
                  },
                  body: JSON.stringify({ preview_token: preview.access_token }),
                  signal: AbortSignal.timeout(5000),
                },
              );
              if (!rollback.ok)
                logger.error("staff_preview_rollback_failed", {
                  status: rollback.status,
                });
            } catch {
              logger.error("staff_preview_rollback_failed", {});
            }
            return problemResponse(500, "Preview session failed", request);
          }
          // Explicit allowlist: no backend token or newly added backend field.
          return Response.json({
            expires_in: preview.expires_in,
            target_account_id: preview.target_account_id,
            target_name: preview.target_name,
          });
        },
      },
      session.user.token,
    );
  },
);
