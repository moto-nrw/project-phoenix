// app/api/timetable/templates/[id]/students/[studentId]/kept-days/route.ts
//
// PUT /api/timetable/templates/{id}/students/{studentId}/kept-days
//   { weekdays, booking_ended_on, kept } — „Bleibt so“ (#5102): das Kind
//   bleibt ohne passende Buchung eingeplant, oder die Rücknahme.
import { proxyPut } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

export const PUT = proxyPut(
  (params) =>
    `/api/timetable/templates/${requirePathSegmentParam(params)}/students/${requirePathSegmentParam(params, "studentId")}/kept-days`,
);
