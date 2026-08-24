import { proxyDelete } from "@/lib/route-proxy.server";
import { requirePathSegmentParam } from "@/lib/route-wrapper-utils.server";

export const DELETE = proxyDelete(
  (params) =>
    `/api/students/care-withdrawals/${requirePathSegmentParam(params, "completionId")}`,
);
