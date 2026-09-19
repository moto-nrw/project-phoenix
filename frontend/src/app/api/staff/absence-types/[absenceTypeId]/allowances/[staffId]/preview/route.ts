import { proxyGet } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

/**
 * GET /api/staff/absence-types/{absenceTypeId}/allowances/{staffId}/preview
 * Which yearly Kontingent a planned booking uses (#3257). Forwards
 * date_start, date_end and half_day.
 */
export const GET = proxyGet(
  (p) =>
    `/api/absence-types/${requirePathSegmentParam(p, "absenceTypeId")}/allowances/${requirePathSegmentParam(p, "staffId")}/preview`,
);
