import { proxyGet, proxyPut } from "@/lib/route-proxy.server";
import { requirePathSegmentParam } from "@/lib/route-wrapper-utils.server";

// GET /api/students/[id]/pickup-schedules - Get pickup schedules and exceptions for a student
export const GET = proxyGet(
  (p) => `/api/students/${requirePathSegmentParam(p)}/pickup-schedules`,
);

// PUT /api/students/[id]/pickup-schedules - Bulk update weekly pickup schedules
export const PUT = proxyPut(
  (p) => `/api/students/${requirePathSegmentParam(p)}/pickup-schedules`,
);
