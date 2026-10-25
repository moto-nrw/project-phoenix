import { proxyDelete, proxyPut } from "~/lib/school/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// PUT /api/school/class-day/day-hints/[schoolClass]/[date]/students/[studentId]:
// den Hinweis der Lehrkraft für heute an die OGS für ein Kind der Klasse
// schreiben (#4795).
export const PUT = proxyPut(
  (p) =>
    `/school/class-day/day-hints/${requirePathSegmentParam(p, "schoolClass")}/${requirePathSegmentParam(p, "date")}/students/${requirePathSegmentParam(p, "studentId")}`,
);

// DELETE: den eigenen Hinweis für das Kind wieder löschen.
export const DELETE = proxyDelete(
  (p) =>
    `/school/class-day/day-hints/${requirePathSegmentParam(p, "schoolClass")}/${requirePathSegmentParam(p, "date")}/students/${requirePathSegmentParam(p, "studentId")}`,
);
