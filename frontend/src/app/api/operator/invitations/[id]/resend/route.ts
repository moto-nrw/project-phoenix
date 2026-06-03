import { createOperatorProxyMethodHandler } from "~/lib/operator/route-wrapper.server";

/**
 * POST /api/operator/invitations/[id]/resend
 * Proxies to backend POST /operator/invitations/{id}/resend.
 * Forwards client IP and User-Agent for audit logging.
 */
export const POST = createOperatorProxyMethodHandler(
  "POST",
  (params) => `/operator/invitations/${String(params.id)}/resend`,
);
