import { proxyGet, proxyPut } from "@/lib/route-proxy.server";
import { requirePathSegmentParam } from "@/lib/route-wrapper-utils.server";

// GET /api/students/[id]/arrival-schedules - Fetch arrival schedules, exceptions and notes
export const GET = proxyGet(
  (p) => `/api/students/${requirePathSegmentParam(p)}/arrival-schedules`,
);

// PUT /api/students/[id]/arrival-schedules - Bulk update weekly arrival schedules
export const PUT = proxyPut(
  (p) => `/api/students/${requirePathSegmentParam(p)}/arrival-schedules`,
);
