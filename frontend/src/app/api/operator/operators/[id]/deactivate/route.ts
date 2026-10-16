import { createOperatorProxyMethodHandler } from "~/lib/operator/route-wrapper.server";

/**
 * POST /api/operator/operators/[id]/deactivate
 * Proxies to backend POST /operator/operators/{id}/deactivate.
 * Forwards client IP and User-Agent for audit logging.
 */
export const POST = createOperatorProxyMethodHandler(
  "POST",
  (params) => `/operator/operators/${String(params.id)}/deactivate`,
);
