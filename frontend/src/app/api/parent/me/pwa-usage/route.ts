import { proxyPost } from "~/lib/parent/route-wrapper.server";

/**
 * POST /api/parent/me/pwa-usage — report that this authenticated parent
 * session runs in PWA standalone display mode (#2189). No body; fans out to
 * every school the guardian account is linked to.
 */
export const POST = proxyPost<null>("/parent/me/pwa-usage");
