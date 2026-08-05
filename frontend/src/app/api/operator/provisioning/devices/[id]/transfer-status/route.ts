import { proxyGet } from "~/lib/operator/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

export const GET = proxyGet(
  (params) =>
    `/operator/devices/${requirePathSegmentParam(params)}/transfer-status`,
);
