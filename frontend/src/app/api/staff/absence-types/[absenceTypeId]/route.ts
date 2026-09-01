import { proxyPut } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

interface AbsenceTypeBody {
  name?: string;
  is_active?: boolean;
  allowance_enabled?: boolean;
  overrun_policy?: "warn" | "block";
}

/**
 * PUT /api/staff/absence-types/{absenceTypeId}
 * Rename or (de)activate a school-defined Abwesenheitsart. There is no DELETE
 * on purpose — an art that was used stays readable on its absences.
 */
export const PUT = proxyPut<unknown, AbsenceTypeBody>(
  (p) => `/api/absence-types/${requirePathSegmentParam(p, "absenceTypeId")}`,
);
