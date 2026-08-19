import { proxyPost } from "~/lib/route-proxy.server";

/**
 * POST /api/pwa/usage — report that this authenticated staff session runs in
 * PWA standalone display mode (#2189). No body; the account comes from the
 * session token.
 */
export const POST = proxyPost<null>("/api/pwa/usage");
