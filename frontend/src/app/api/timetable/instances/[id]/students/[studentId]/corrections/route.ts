import { proxyGet } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

export const GET = proxyGet(
  (params) =>
    `/api/timetable/instances/${requirePathSegmentParam(params)}/students/${requirePathSegmentParam(params, "studentId")}/corrections`,
);
