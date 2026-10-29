import { proxyPost } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

/**
 * POST /api/students/[id]/accident-reports
 * Records an accident report (Unfallanzeige, #4771). The backend owns the
 * student_accidents:manage gate, the field rules and the audit row.
 */
export const POST = proxyPost(
  (p) => `/api/students/${requirePathSegmentParam(p)}/accident-reports`,
);
