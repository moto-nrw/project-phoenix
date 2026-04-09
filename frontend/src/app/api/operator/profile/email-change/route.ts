import { createOperatorProxyPostHandler } from "~/lib/operator/route-wrapper";

/**
 * POST /api/operator/profile/email-change
 * Proxies to backend POST /operator/profile/email-change.
 * Uses the proxy handler to preserve backend error messages verbatim
 * while adding 401 retry with token refresh.
 */
export const POST = createOperatorProxyPostHandler(
  "/operator/profile/email-change",
);
