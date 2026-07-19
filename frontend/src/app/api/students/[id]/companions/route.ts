import { proxyGet } from "@/lib/route-proxy.server";
import { requirePathSegmentParam } from "@/lib/route-wrapper-utils.server";

// GET /api/students/[id]/companions - Children this child walks home with.
// Read-only on purpose: companions are written through PUT /api/students/[id],
// together with the departure plan that makes the link legal.
export const GET = proxyGet(
  (p) => `/api/students/${requirePathSegmentParam(p)}/companions`,
);
