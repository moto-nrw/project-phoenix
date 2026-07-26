// app/api/timetable/templates/[id]/split/route.ts
//
// POST /api/timetable/templates/{id}/split
//   "Ab Datum ändern": ends the old template version at effective_date and
//   creates a new version with the submitted fields. Body shape mirrors
//   SplitTemplateBody in lib/timetable-types.ts.
//
// The Go backend wraps responses in { status, data, message }. proxyPost
// strips that envelope so route-wrapper does not double-wrap the payload.
import { proxyPost } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

export const POST = proxyPost(
  (params) =>
    `/api/timetable/templates/${requirePathSegmentParam(params)}/split`,
);
