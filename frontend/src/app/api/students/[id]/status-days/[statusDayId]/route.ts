import { proxyDelete } from "~/lib/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

export const DELETE = proxyDelete(
  (p) =>
    `/api/students/${requirePathSegmentParam(p)}/status-days/${requirePathSegmentParam(p, "statusDayId")}`,
);
