import { proxyGet } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

/**
 * Proxy GET /api/students/{id}/parent-notes → backend. Returns the newest
 * notes a guardian left for the student via the parents portal. Backend
 * enforces staff read access to the student.
 */
export const GET = proxyGet(
  (p) => `/api/students/${requirePathSegmentParam(p)}/parent-notes`,
);
