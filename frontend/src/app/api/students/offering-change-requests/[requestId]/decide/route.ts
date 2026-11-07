import { proxyPost } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Approves (applies the dated offering switch) or rejects one request (#1665).
export const POST = proxyPost(
  (p) =>
    `/api/students/offering-change-requests/${requirePathSegmentParam(p, "requestId")}/decide`,
);
