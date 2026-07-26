import { proxyGet } from "~/lib/route-proxy.server";

/**
 * GET /api/time-tracking/current
 * Get current active work session
 */
export const GET = proxyGet("/api/time-tracking/current");
