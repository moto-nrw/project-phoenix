import { proxyPut } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

/**
 * PUT /api/students/[id]/accident-reports/[documentId]
 * Changes an open accident report (#4771); a signed one answers 409.
 */
export const PUT = proxyPut(
  (p) =>
    `/api/students/${requirePathSegmentParam(p)}/accident-reports/${requirePathSegmentParam(p, "documentId")}`,
);
