import { proxyPatch } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

export const PATCH = proxyPatch(
  (params) =>
    `/api/timetable/instances/${requirePathSegmentParam(params)}/students/${requirePathSegmentParam(params, "studentId")}`,
);
