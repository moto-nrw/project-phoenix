// GET /api/timetable/instances/[id]/participants — Teilnehmer-Namen eines
// Blocks für die Leseansicht (#2283): schedules:read genügt, das Backend
// filtert pro Kind über CanReadStudent (gdpr.student_data_scope).
import { proxyGet } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

export const GET = proxyGet(
  (params) =>
    `/api/timetable/instances/${requirePathSegmentParam(params)}/participants`,
);
