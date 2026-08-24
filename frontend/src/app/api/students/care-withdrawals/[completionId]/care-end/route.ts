import { proxyPost } from "@/lib/route-proxy.server";
import { requirePathSegmentParam } from "@/lib/route-wrapper-utils.server";

export const POST = proxyPost(
  (params) =>
    `/api/students/care-withdrawals/${requirePathSegmentParam(params, "completionId")}/care-end`,
);
