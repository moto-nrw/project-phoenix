import type { NextRequest } from "next/server";
import { tenantPortal } from "~/server/auth";
import { sessionMutationOriginError } from "~/server/auth/same-origin";
import { problemResponse } from "~/lib/api-response-error.server";
import { forwardJsonRequest } from "~/lib/json-proxy.server";
import { tenant } from "~/lib/route-wrapper.server";
import type { RouteContext } from "~/lib/route-wrapper-utils.server";
import { installMintedSession } from "~/lib/session-transition.server";

export const POST = tenantPortal.withAuth(
  async (request: NextRequest, context: RouteContext) => {
    const originError = sessionMutationOriginError(request);
    if (originError) return originError;
    if ((await tenantPortal.auth())?.user?.isPreview)
      return problemResponse(409, "A preview is already running", request);
    return forwardJsonRequest(
      request,
      context,
      {
        method: "POST",
        path: "/auth/staff-preview",
        onSuccess: async (response) => {
          const preview = (await response.json().catch(() => {
            throw new Error("Invalid session transition response");
          })) as {
            access_token: string;
            expires_in: number;
            target_account_id: string;
            target_name: string;
          };
          const failure = await installMintedSession(
            request,
            () =>
              tenantPortal.setStaffPreviewSession({
                accessToken: preview.access_token,
                expiresIn: preview.expires_in,
                targetAccountId: preview.target_account_id,
                targetName: preview.target_name,
              }),
            {
              path: "/auth/staff-preview/end",
              json: { preview_token: preview.access_token },
            },
          );
          if (failure) return failure;
          // Explicit allowlist: no backend token or newly added backend field.
          return Response.json({
            expires_in: preview.expires_in,
            target_account_id: preview.target_account_id,
            target_name: preview.target_name,
          });
        },
      },
      (path, call) =>
        tenant.backendFetch(request, path, {
          ...call,
          retry: {
            off: "its 401 names why the preview is refused, like the school switch",
          },
        }),
    );
  },
);
