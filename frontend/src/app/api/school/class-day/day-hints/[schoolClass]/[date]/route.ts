import { proxyDelete, proxyPut } from "~/lib/school/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// PUT /api/school/class-day/day-hints/[schoolClass]/[date]: den Hinweis der
// Lehrkraft für heute an die OGS für die ganze Klasse schreiben (#4795).
export const PUT = proxyPut(
  (p) =>
    `/school/class-day/day-hints/${requirePathSegmentParam(p, "schoolClass")}/${requirePathSegmentParam(p, "date")}`,
);

// DELETE: den eigenen Hinweis für die Klasse wieder löschen.
export const DELETE = proxyDelete(
  (p) =>
    `/school/class-day/day-hints/${requirePathSegmentParam(p, "schoolClass")}/${requirePathSegmentParam(p, "date")}`,
);
