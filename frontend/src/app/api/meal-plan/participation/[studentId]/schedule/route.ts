import { proxyPut } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Feste Tage eines Kindes ab einem Tag setzen (#3873). meals:orders erzwingt
// das Backend.
export const PUT = proxyPut(
  (p) =>
    `/api/meal-plan/participation/${requirePathSegmentParam(p, "studentId")}/schedule`,
);
