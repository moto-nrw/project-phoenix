import { createOperatorPublicProxyPostHandler } from "~/lib/operator/route-wrapper.server";
import { OPERATOR_INVITATION_COOKIE } from "~/lib/operator/operator-invitation-session.server";

/**
 * POST /api/operator/auth/invitations/validate
 * Public endpoint (no auth required) — proxies to backend POST /operator/auth/invitations/validate.
 * Token is sent in the request body, never in the URL.
 */
export const POST = createOperatorPublicProxyPostHandler(
  "/operator/auth/invitations/validate",
  {
    transformBody(request, body) {
      const token = request.cookies.get(OPERATOR_INVITATION_COOKIE)?.value;
      return {
        ...(typeof body === "object" && body !== null ? body : {}),
        token: token ?? "",
      };
    },
  },
);
