import { proxyPut } from "~/lib/operator/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

export const PUT = proxyPut(
  (params) => `/operator/suggestions/${requirePathSegmentParam(params)}/hidden`,
);
