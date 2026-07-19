import { proxyGet, proxyPut } from "@/lib/route-proxy.server";
import { requirePathSegmentParam } from "@/lib/route-wrapper-utils.server";

// GET /api/students/[id]/companions - Children this child walks home with
export const GET = proxyGet(
  (p) => `/api/students/${requirePathSegmentParam(p)}/companions`,
);

// PUT /api/students/[id]/companions - Replace the complete "läuft mit" list
export const PUT = proxyPut(
  (p) => `/api/students/${requirePathSegmentParam(p)}/companions`,
);
