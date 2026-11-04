import { proxyPost } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

/**
 * POST /api/students/[id]/accident-reports/[documentId]/signature
 * The leadership signs an open accident report (#4771) with the shared
 * signature field (#4785).
 */
export const POST = proxyPost(
  (p) =>
    `/api/students/${requirePathSegmentParam(p)}/accident-reports/${requirePathSegmentParam(p, "documentId")}/signature`,
);
