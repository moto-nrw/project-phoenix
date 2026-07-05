import { proxyPut } from "@/lib/route-proxy.server";
import { requirePathSegmentParam } from "@/lib/route-wrapper-utils.server";

// PUT /api/guardians/relationships/[relationshipId] - Update student-guardian relationship
export const PUT = proxyPut(
  (p) =>
    `/api/guardians/relationships/${requirePathSegmentParam(p, "relationshipId")}`,
);
