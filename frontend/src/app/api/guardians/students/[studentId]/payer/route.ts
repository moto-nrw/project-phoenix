import { proxyPut } from "@/lib/route-proxy.server";
import { requirePathSegmentParam } from "@/lib/route-wrapper-utils.server";

// PUT /api/guardians/students/[studentId]/payer - mark which guardian pays for
// this child, or clear the assignment with a null guardian_id.
export const PUT = proxyPut(
  (p) =>
    `/api/guardians/students/${requirePathSegmentParam(p, "studentId")}/payer`,
);
