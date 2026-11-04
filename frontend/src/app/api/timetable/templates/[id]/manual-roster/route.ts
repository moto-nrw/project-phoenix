// app/api/timetable/templates/[id]/manual-roster/route.ts
//
// PUT /api/timetable/templates/{id}/manual-roster  { confirmed }
//   „Liste so lassen“ (#5103): hides the deviation hint for the
//   series, or shows it again.
import { proxyPut } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

export const PUT = proxyPut(
  (params) =>
    `/api/timetable/templates/${requirePathSegmentParam(params)}/manual-roster`,
);
