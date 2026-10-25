import { proxyGet } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Anmeldung eines Kindes zum Mittagessen für die Schule (#3873). meals:register
// erzwingt das Backend.
export const GET = proxyGet(
  (p) =>
    `/api/meal-plan/participation/${requirePathSegmentParam(p, "studentId")}`,
);
