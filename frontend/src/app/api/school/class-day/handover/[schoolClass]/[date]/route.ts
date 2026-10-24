import { proxyDelete, proxyPut } from "~/lib/school/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// PUT /api/school/class-day/handover/[schoolClass]/[date]: eine zugewiesene
// Klasse heute an die OGS übergeben, mit den Kindern, die nicht da sind (#4794).
export const PUT = proxyPut(
  (p) =>
    `/school/class-day/handover/${requirePathSegmentParam(p, "schoolClass")}/${requirePathSegmentParam(p, "date")}`,
);

// DELETE: die Übergabe am selben Tag zurücknehmen.
export const DELETE = proxyDelete(
  (p) =>
    `/school/class-day/handover/${requirePathSegmentParam(p, "schoolClass")}/${requirePathSegmentParam(p, "date")}`,
);
