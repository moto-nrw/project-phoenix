import { createOperatorPublicProxyPostHandler } from "~/lib/operator/route-wrapper.server";
import { operatorInvitationToken } from "~/lib/operator/operator-invitation-session.server";

/**
 * POST /api/operator/auth/invitations/validate
 * Public endpoint (no auth required) — proxies to backend POST /operator/auth/invitations/validate.
 * Token is sent in the request body, never in the URL.
 */
export const POST = createOperatorPublicProxyPostHandler(
  "/operator/auth/invitations/validate",
  {
    transformBody(request, body) {
      return {
        ...(typeof body === "object" && body !== null ? body : {}),
        token: operatorInvitationToken(request),
      };
    },
  },
);
