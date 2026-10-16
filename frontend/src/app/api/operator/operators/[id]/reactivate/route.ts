import { createOperatorProxyMethodHandler } from "~/lib/operator/route-wrapper.server";

/**
 * POST /api/operator/operators/[id]/reactivate
 * Proxies to backend POST /operator/operators/{id}/reactivate.
 * Forwards client IP and User-Agent for audit logging.
 */
export const POST = createOperatorProxyMethodHandler(
  "POST",
  (params) => `/operator/operators/${String(params.id)}/reactivate`,
);
