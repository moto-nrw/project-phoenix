import { proxyPost } from "~/lib/route-proxy.server";

interface CheckInRequest {
  status: "present" | "home_office";
}

/**
 * POST /api/time-tracking/check-in
 * Check in for work with specified status
 */
export const POST = proxyPost<unknown, CheckInRequest>(
  "/api/time-tracking/check-in",
);
