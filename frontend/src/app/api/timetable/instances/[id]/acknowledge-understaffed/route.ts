// POST /api/timetable/instances/[id]/acknowledge-understaffed — marks a block
// as deliberately left unstaffed (or clears it) for the Vertretungsplan
// (#1840). Body: { ack: boolean, note?: string }.
import { proxyPost } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

export const POST = proxyPost(
  (params) =>
    `/api/timetable/instances/${requirePathSegmentParam(params)}/acknowledge-understaffed`,
);
