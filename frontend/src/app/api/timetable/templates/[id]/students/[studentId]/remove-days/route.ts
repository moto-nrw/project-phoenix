// app/api/timetable/templates/[id]/students/[studentId]/remove-days/route.ts
//
// POST /api/timetable/templates/{id}/students/{studentId}/remove-days
//   „Dort auch entfernen“ (#5102): nimmt ein Kind ab dem Stichtag an den
//   übergebenen Wochentagen aus einem von Hand gepflegten Regeltermin.
import { proxyPost } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

export const POST = proxyPost(
  (params) =>
    `/api/timetable/templates/${requirePathSegmentParam(params)}/students/${requirePathSegmentParam(params, "studentId")}/remove-days`,
);
