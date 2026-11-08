import { proxyPost } from "~/lib/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Closes a request that only concerns past days (#2267): nothing is applied.
// 409 students.request_not_past: it still concerns coming days;
// students.change_request_stale: it changed in the meantime.
export const POST = proxyPost(
  (p) =>
    `/api/students/change-requests/${requirePathSegmentParam(p, "kind")}/${requirePathSegmentParam(p)}/mark-done`,
);
