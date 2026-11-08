import { createPublicJsonProxy } from "~/lib/backend-proxy-route.server";
import { forwardBackendResponse } from "~/lib/backend-proxy-response.server";
import { operatorJsonErrors } from "~/lib/operator/route-wrapper.server";
import {
  operatorInvitationCookieName,
  operatorInvitationCookieOptions,
  operatorInvitationFlowID,
  operatorInvitationToken,
} from "~/lib/operator/operator-invitation-session.server";

/**
 * Public: the invitation token travels in the body, taken from the
 * invitation flow's cookie, never in the URL. An accepted invitation ends
 * that cookie.
 */
export const POST = createPublicJsonProxy({
  method: "POST",
  path: "/operator/auth/invitations/accept",
  ...operatorJsonErrors,
  mapJsonBody: (body, request) => ({
    ...(typeof body === "object" && body !== null ? body : {}),
    token: operatorInvitationToken(request),
  }),
  onSuccess: (backend, request) => {
    const response = forwardBackendResponse(backend, request);
    response.cookies.set(
      operatorInvitationCookieName(operatorInvitationFlowID(request)),
      "",
      { ...operatorInvitationCookieOptions, maxAge: 0 },
    );
    return response;
  },
});
