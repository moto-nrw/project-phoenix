import { proxyGet, proxyDelete } from "~/lib/operator/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

export const GET = proxyGet(
  (params) => `/operator/suggestions/${requirePathSegmentParam(params)}`,
);

export const DELETE = proxyDelete(
  (params) => `/operator/suggestions/${requirePathSegmentParam(params)}`,
);
