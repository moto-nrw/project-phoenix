import { createOperatorPublicProxyPostHandler } from "~/lib/operator/route-wrapper.server";
import {
  OPERATOR_INVITATION_COOKIE,
  operatorInvitationCookieOptions,
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
      const token = request.cookies.get(OPERATOR_INVITATION_COOKIE)?.value;
      return {
        ...(typeof body === "object" && body !== null ? body : {}),
        token: token ?? "",
      };
    },
    decorateResponse(_request, response, backendSucceeded) {
      if (!backendSucceeded) return;
      response.cookies.set(OPERATOR_INVITATION_COOKIE, "", {
        ...operatorInvitationCookieOptions,
        maxAge: 0,
      });
    },
  },
);
