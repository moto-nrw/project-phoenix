// POST /api/timetable/instances/[id]/deviations — applies a whole
// Vertretungsplan slide-over save (absences, substitute, understaffed
// acknowledgement, cancel) atomically in one backend transaction (#1840).
import { proxyPost } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

export const POST = proxyPost(
  (params) =>
    `/api/timetable/instances/${requirePathSegmentParam(params)}/deviations`,
);
