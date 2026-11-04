// app/api/timetable/templates/[id]/offering-connection/route.ts
//
// POST /api/timetable/templates/{id}/offering-connection
//   „Mit Angebot verbinden“ (#5103): every period of the series from
//   effective_date on takes its children from the offering.
import { proxyPost } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

export const POST = proxyPost(
  (params) =>
    `/api/timetable/templates/${requirePathSegmentParam(params)}/offering-connection`,
);
