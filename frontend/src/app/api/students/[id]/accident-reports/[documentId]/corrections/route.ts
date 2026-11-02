import { proxyPost } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

/**
 * POST /api/students/[id]/accident-reports/[documentId]/corrections
 * Records a new version of a signed accident report (#4771); the signed one
 * stays as it is.
 */
export const POST = proxyPost(
  (p) =>
    `/api/students/${requirePathSegmentParam(p)}/accident-reports/${requirePathSegmentParam(p, "documentId")}/corrections`,
);
