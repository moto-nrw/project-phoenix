// POST /api/timetable/instances/[id]/start — transitions a planned
// instance to active and creates the active.group bridge.
import { proxyPost } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

export const POST = proxyPost(
  (params) =>
    `/api/timetable/instances/${requirePathSegmentParam(params)}/start`,
);
