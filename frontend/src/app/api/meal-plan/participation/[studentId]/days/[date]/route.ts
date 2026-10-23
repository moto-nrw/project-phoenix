import { proxyDelete, proxyPut } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Einen Tag an- oder abmelden bzw. zurück auf die festen Tage (#3873), auch
// nach der Frist. meals:register erzwingt das Backend.
export const PUT = proxyPut(
  (p) =>
    `/api/meal-plan/participation/${requirePathSegmentParam(p, "studentId")}/days/${requirePathSegmentParam(p, "date")}`,
);
export const DELETE = proxyDelete(
  (p) =>
    `/api/meal-plan/participation/${requirePathSegmentParam(p, "studentId")}/days/${requirePathSegmentParam(p, "date")}`,
);
