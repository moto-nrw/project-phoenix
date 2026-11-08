import { proxyDelete, proxyPut } from "~/lib/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

export const PUT = proxyPut(
  (params) => `/api/timetable/instances/${requirePathSegmentParam(params)}`,
);

export const DELETE = proxyDelete(
  (params) => `/api/timetable/instances/${requirePathSegmentParam(params)}`,
);
