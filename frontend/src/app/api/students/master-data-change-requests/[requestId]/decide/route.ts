import { proxyPost } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Approves (and applies) or rejects one master-data change request.
export const POST = proxyPost(
  (p) =>
    `/api/students/master-data-change-requests/${requirePathSegmentParam(p, "requestId")}/decide`,
);
