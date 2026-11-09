import { proxyPost } from "~/lib/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Approves (applies the weekly plan) or rejects one care-schedule request (#1803).
export const POST = proxyPost(
  (p) =>
    `/api/students/care-schedule-change-requests/${requirePathSegmentParam(p, "requestId")}/decide`,
);
