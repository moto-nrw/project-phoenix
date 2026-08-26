// GET /api/timetable/instances/[id]/guardian-notice — preview for "Eltern
// informieren" in the cancel dialog (#2601): whether the school allows the
// notice, whether the checkbox starts ticked, and how many children /
// families the block reaches.
import { proxyGet } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

export const GET = proxyGet(
  (params) =>
    `/api/timetable/instances/${requirePathSegmentParam(params)}/guardian-notice`,
);
