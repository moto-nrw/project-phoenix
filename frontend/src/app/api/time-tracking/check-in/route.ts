import { proxyPost } from "~/lib/route-proxy.server";

interface CheckInRequest {
  status: "present" | "home_office";
  /** Optional F9 deviation reason, sent after a deviation_reason_required conflict. */
  reason?: string;
}

/**
 * POST /api/time-tracking/check-in
 * Check in for work with specified status
 */
export const POST = proxyPost<unknown, CheckInRequest>(
  "/api/time-tracking/check-in",
);
