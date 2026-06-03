import { createOperatorPublicProxyPostHandler } from "~/lib/operator/route-wrapper.server";

/**
 * POST /api/operator/auth/invitations/accept
 * Public endpoint (no auth required) — proxies to backend POST /operator/auth/invitations/accept.
 * Token is sent in the request body, never in the URL.
 */
export const POST = createOperatorPublicProxyPostHandler(
  "/operator/auth/invitations/accept",
);
