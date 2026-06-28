import { proxyDelete, proxyGet, proxyPut } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

export const GET = proxyGet(
  (params) => `/api/timetable/templates/${requirePathSegmentParam(params)}`,
);

export const PUT = proxyPut(
  (params) => `/api/timetable/templates/${requirePathSegmentParam(params)}`,
);

export const DELETE = proxyDelete(
  (params) => `/api/timetable/templates/${requirePathSegmentParam(params)}`,
);
