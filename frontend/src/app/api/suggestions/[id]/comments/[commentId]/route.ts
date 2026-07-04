import { proxyDelete } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

export const DELETE = proxyDelete(
  (params) =>
    `/api/suggestions/${requirePathSegmentParam(params)}/comments/${requirePathSegmentParam(params, "commentId")}`,
);
