import { proxyGet } from "@/lib/route-proxy.server";
import { requirePathSegmentParam } from "@/lib/route-wrapper-utils.server";

export const GET = proxyGet(
  (params) =>
    `/api/students/care-withdrawals/${requirePathSegmentParam(params, "completionId")}/deletion-impact`,
);
