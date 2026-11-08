import { createPublicJsonProxy } from "~/lib/backend-proxy-route.server";
import { operatorJsonErrors } from "~/lib/operator/route-wrapper.server";
import { operatorInvitationToken } from "~/lib/operator/operator-invitation-session.server";

/**
 * Public: the invitation token travels in the body, taken from the
 * invitation flow's cookie, never in the URL.
 */
export const POST = createPublicJsonProxy({
  method: "POST",
  path: "/operator/auth/invitations/validate",
  ...operatorJsonErrors,
  mapJsonBody: (body, request) => ({
    ...(typeof body === "object" && body !== null ? body : {}),
    token: operatorInvitationToken(request),
  }),
});
