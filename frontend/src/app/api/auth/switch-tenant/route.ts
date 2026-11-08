import type { NextRequest } from "next/server";
import { tenantPortal } from "~/server/auth";
import { PORTALS } from "~/server/auth/portals";
import { sessionMutationOriginError } from "~/server/auth/same-origin";
import { forwardJsonRequest } from "~/lib/json-proxy.server";
import { tenant } from "~/lib/route-wrapper.server";
import type { RouteContext } from "~/lib/route-wrapper-utils.server";
import {
  installMintedSession,
  logoutRollback,
  mintedPair,
} from "~/lib/session-transition.server";

export const POST = tenantPortal.withAuth(
  async (request: NextRequest, context: RouteContext) => {
    const originError = sessionMutationOriginError(request);
    if (originError) return originError;
    return forwardJsonRequest(
      request,
      context,
      {
        method: "POST",
        path: "/auth/switch-tenant",
        onSuccess: async (response) => {
          const pair = mintedPair(
            (await response.json().catch(() => {
              throw new Error("Invalid session transition response");
            })) as Record<string, unknown>,
          );
          return (
            (await installMintedSession(
              request,
              () =>
                tenantPortal.setSession(pair.access_token, pair.refresh_token),
              logoutRollback(PORTALS.tenant.backend.logout, pair),
            )) ?? Response.json({ success: true })
          );
        },
      },
      (path, call) =>
        tenant.backendFetch(request, path, {
          ...call,
          retry: {
            off: "its 401 names why the switch is refused (identity.mfa_session_required)",
          },
        }),
    );
  },
);
