import { proxyPut } from "~/lib/school/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// PUT /api/school/class-day/morning-attendance/[schoolClass]/[date]:
// Anwesenheit am Vormittag einer zugewiesenen Klasse für heute (#4790).
export const PUT = proxyPut(
  (p) =>
    `/school/class-day/morning-attendance/${requirePathSegmentParam(p, "schoolClass")}/${requirePathSegmentParam(p, "date")}`,
);
