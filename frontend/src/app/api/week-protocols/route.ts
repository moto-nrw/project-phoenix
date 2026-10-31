import { proxyGet } from "~/lib/route-proxy.server";

/**
 * GET /api/week-protocols
 * Lists the weekly protocols of the groups (#4769), optionally of one group
 * (`?group_id=`). The backend narrows the list to the caller's groups unless
 * the caller leads the school.
 */
export const GET = proxyGet(() => "/api/week-protocols");
