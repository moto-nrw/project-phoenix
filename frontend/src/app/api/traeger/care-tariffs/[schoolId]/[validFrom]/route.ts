import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyDelete } from "~/lib/traeger/route-wrapper.server";

// Preise einer Schule ab einem Monat löschen, solange keine Rechnung den
// Monat abrechnet (#4684).
export const DELETE = proxyDelete(
  (params) =>
    `/traeger/care-tariffs/${requirePathSegmentParam(params, "schoolId")}/${requirePathSegmentParam(params, "validFrom")}`,
);
