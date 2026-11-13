import { proxyPost } from "~/lib/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// POST /api/work-time-models/{id}/archive (#6891).
export const POST = proxyPost(
  (params) =>
    `/api/work-time-models/${requirePathSegmentParam(params)}/archive`,
);
