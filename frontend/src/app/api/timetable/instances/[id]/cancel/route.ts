// POST /api/timetable/instances/[id]/cancel — cancels a planned or active
// instance; from active, ends the bridge cleanly via
// active.EndActivitySession.
import { proxyPost } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

export const POST = proxyPost(
  (params) =>
    `/api/timetable/instances/${requirePathSegmentParam(params)}/cancel`,
);
