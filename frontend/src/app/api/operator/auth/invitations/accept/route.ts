import { createOperatorPublicProxyPostHandler } from "~/lib/operator/route-wrapper.server";
import {
  operatorInvitationCookieName,
  operatorInvitationCookieOptions,
  operatorInvitationFlowID,
  operatorInvitationToken,
} from "~/lib/operator/operator-invitation-session.server";

/**
 * POST /api/operator/auth/invitations/accept
 * Public endpoint (no auth required) — proxies to backend POST /operator/auth/invitations/accept.
 * Token is sent in the request body, never in the URL.
 */
export const POST = createOperatorPublicProxyPostHandler(
  "/operator/auth/invitations/accept",
  {
    transformBody(request, body) {
      return {
        ...(typeof body === "object" && body !== null ? body : {}),
        token: operatorInvitationToken(request),
      };
    },
    decorateResponse(request, response, backendSucceeded) {
      if (!backendSucceeded) return;
      response.cookies.set(
        operatorInvitationCookieName(operatorInvitationFlowID(request)),
        "",
        {
          ...operatorInvitationCookieOptions,
          maxAge: 0,
        },
      );
    },
  },
);
