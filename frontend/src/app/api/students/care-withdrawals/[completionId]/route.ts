import { proxyDelete } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// The body carries the deletion confirmation (fingerprint, name, reason).
export const DELETE = proxyDelete(
  (p) =>
    `/api/students/care-withdrawals/${requirePathSegmentParam(p, "completionId")}`,
  { withBody: true },
);
