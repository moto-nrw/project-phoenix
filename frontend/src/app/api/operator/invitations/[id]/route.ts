import { createOperatorProxyMethodHandler } from "~/lib/operator/route-wrapper.server";

/**
 * DELETE /api/operator/invitations/[id]
 * Proxies to backend DELETE /operator/invitations/{id}.
 * Forwards client IP and User-Agent for audit logging.
 */
export const DELETE = createOperatorProxyMethodHandler(
  "DELETE",
  (params) => `/operator/invitations/${String(params.id)}`,
);
