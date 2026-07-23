// POST /api/timetable/instances/[id]/move-staff — atomarer Personal-Move
// (#1884): Entnahme aus dem Quellblock und Zuordnung zum Zielblock in EINEM
// Save; ohne source_instance_id wird eine freie Person aus dem Pool zugewiesen.
import { proxyPost } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Dokumentiert den erwarteten Body (Snake-Case Richtung Backend).
interface MoveStaffBody {
  staff_id: number;
  source_instance_id?: number;
}

export const POST = proxyPost<unknown, MoveStaffBody>(
  (params) =>
    `/api/timetable/instances/${requirePathSegmentParam(params)}/move-staff`,
);
