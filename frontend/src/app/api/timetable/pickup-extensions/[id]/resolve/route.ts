// app/api/timetable/pickup-extensions/[id]/resolve/route.ts
//
// POST /api/timetable/pickup-extensions/{id}/resolve — adds the child to the
// chosen blocks, or to none, and closes the decision (#3261).
import { proxyPost } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

export const POST = proxyPost(
  (params) =>
    `/api/timetable/pickup-extensions/${requirePathSegmentParam(params, "id")}/resolve`,
);
