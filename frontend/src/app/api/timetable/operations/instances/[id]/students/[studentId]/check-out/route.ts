import { proxyPost } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

export const POST = proxyPost(
  (params) =>
    `/api/timetable/operations/instances/${requirePathSegmentParam(params)}/students/${requirePathSegmentParam(params, "studentId")}/check-out`,
);
