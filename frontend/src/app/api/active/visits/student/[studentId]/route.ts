// app/api/active/visits/student/[studentId]/route.ts
import { proxyGet } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

/**
 * Handler for GET /api/active/visits/student/[studentId]
 * Returns visits for a specific student
 */
export const GET = proxyGet(
  (p) =>
    `/api/active/visits/student/${requirePathSegmentParam(p, "studentId")}`,
  { raw: true },
);
