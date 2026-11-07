import { proxyPost } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Replaces a decision already taken (#2267); the old one stays in the history.
// 409 students.correction_unsupported: this kind cannot be corrected;
// students.request_not_decided: the request is still open.
export const POST = proxyPost(
  (p) =>
    `/api/students/change-requests/${requirePathSegmentParam(p, "kind")}/${requirePathSegmentParam(p)}/correct`,
);
