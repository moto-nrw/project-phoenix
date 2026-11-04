import { proxyDelete, proxyPut } from "~/lib/school/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

function absenceReportPath(p: Record<string, unknown>): string {
  return `/school/class-day/absence-reports/${requirePathSegmentParam(p, "schoolClass")}/${requirePathSegmentParam(p, "date")}/students/${requirePathSegmentParam(p, "studentId")}`;
}

// PUT /api/school/class-day/absence-reports/[schoolClass]/[date]/students/[studentId]:
// die Schulverwaltung trägt für ein Kind, das ohne Meldung fehlt, „krank“
// oder „entschuldigt“ für heute nach (#4791).
export const PUT = proxyPut(absenceReportPath);

// DELETE: die nachgetragene Meldung zurücknehmen.
export const DELETE = proxyDelete(absenceReportPath);
