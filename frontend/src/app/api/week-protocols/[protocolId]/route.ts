import { proxyGet } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

/**
 * GET /api/week-protocols/[protocolId]
 * One weekly protocol with its days (#4769); the backend records the opening
 * in the data access log and answers 404 outside the caller's reach.
 */
export const GET = proxyGet(
  (p) => `/api/week-protocols/${requirePathSegmentParam(p, "protocolId")}`,
);
