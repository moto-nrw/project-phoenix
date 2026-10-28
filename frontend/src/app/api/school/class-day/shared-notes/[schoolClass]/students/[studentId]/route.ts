import { proxyGet } from "~/lib/school/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// GET /api/school/class-day/shared-notes/[schoolClass]/students/[studentId]:
// die Notizen, die die OGS für die Schule freigegeben hat (#4797). Nur die
// Lehrkraft der Klasse bekommt eine Antwort; jeder Abruf wird protokolliert.
export const GET = proxyGet(
  (p) =>
    `/school/class-day/shared-notes/${requirePathSegmentParam(p, "schoolClass")}/students/${requirePathSegmentParam(p, "studentId")}`,
);
