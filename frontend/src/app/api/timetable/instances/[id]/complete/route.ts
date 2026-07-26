// POST /api/timetable/instances/[id]/complete — transitions an active
// instance to completed; closes the active.group, marks remaining expected
// students as absent.
import { proxyPost } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

export const POST = proxyPost(
  (params) =>
    `/api/timetable/instances/${requirePathSegmentParam(params)}/complete`,
);
