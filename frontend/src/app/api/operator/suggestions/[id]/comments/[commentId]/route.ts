import { proxyDelete } from "~/lib/operator/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

export const DELETE = proxyDelete(
  (params) =>
    `/operator/suggestions/${requirePathSegmentParam(params)}/comments/${requirePathSegmentParam(params, "commentId")}`,
);
