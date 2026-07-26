// app/api/active/visits/student/[studentId]/current/route.ts
import { proxyGet } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

/**
 * Handler for GET /api/active/visits/student/[studentId]/current
 * Returns the current visit for a specific student
 */
export const GET = proxyGet(
  (p) =>
    `/api/active/visits/student/${requirePathSegmentParam(p, "studentId")}/current`,
  { raw: true },
);
