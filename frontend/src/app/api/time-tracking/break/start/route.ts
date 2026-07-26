import { proxyPost } from "~/lib/route-proxy.server";

/**
 * Request body for starting a break
 */
interface StartBreakRequest {
  planned_duration_minutes?: number;
}

/**
 * POST /api/time-tracking/break/start
 * Start a new break for the current session
 */
export const POST = proxyPost<unknown, StartBreakRequest>(
  "/api/time-tracking/break/start",
);
