// app/api/timetable/templates/[id]/offering-preview/route.ts
//
// GET /api/timetable/templates/{id}/offering-preview?offering_ids=...&from=...
//   Per weekday: which children join and which leave when the series takes
//   its children from the chosen offerings (#5103). Read-only.
import { proxyGet } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

export const GET = proxyGet(
  (params) =>
    `/api/timetable/templates/${requirePathSegmentParam(params)}/offering-preview`,
);
