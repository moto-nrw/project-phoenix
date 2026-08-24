import { proxyDelete, proxyPut } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

interface UpdateAbsenceBody {
  absence_type?: string;
  /** Re-points the school-defined Abwesenheitsart; `null` clears it (#2403). */
  absence_type_id?: number | null;
  date_start?: string;
  date_end?: string;
  half_day?: boolean;
  note?: string;
}

/**
 * PUT /api/time-tracking/absences/{id}
 * Update an absence
 */
export const PUT = proxyPut<unknown, UpdateAbsenceBody>(
  (p) => `/api/time-tracking/absences/${requirePathSegmentParam(p)}`,
);

/**
 * DELETE /api/time-tracking/absences/{id}
 * Delete an absence
 */
export const DELETE = proxyDelete(
  (p) => `/api/time-tracking/absences/${requirePathSegmentParam(p)}`,
);
