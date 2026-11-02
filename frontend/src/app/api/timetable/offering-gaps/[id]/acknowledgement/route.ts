// app/api/timetable/offering-gaps/[id]/acknowledgement/route.ts
//
// PUT /api/timetable/offering-gaps/{offeringId}/acknowledgement
//   { period_id, acknowledged } — „Kein eigener Regeltermin nötig“ (#5140)
//   for the classes the hint names, or its undo.
import { proxyPut } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

export const PUT = proxyPut(
  (params) =>
    `/api/timetable/offering-gaps/${requirePathSegmentParam(params)}/acknowledgement`,
);
