import { proxyDelete, proxyPut } from "~/lib/school/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// PUT /api/school/class-day/homework/[schoolClass]/[homeworkId]: einen
// eigenen Hausaufgaben-Eintrag ändern (#4796).
export const PUT = proxyPut(
  (p) =>
    `/school/class-day/homework/${requirePathSegmentParam(p, "schoolClass")}/${requirePathSegmentParam(p, "homeworkId")}`,
);

// DELETE: den eigenen Eintrag löschen.
export const DELETE = proxyDelete(
  (p) =>
    `/school/class-day/homework/${requirePathSegmentParam(p, "schoolClass")}/${requirePathSegmentParam(p, "homeworkId")}`,
);
