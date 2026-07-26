import { proxyDelete, proxyPut } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

interface UpdateShiftBody {
  staff_id?: number;
  date: string;
  start_time: string;
  end_time: string;
  break_minutes: number;
  cancelled?: boolean;
  change_reason?: string;
}

/**
 * PUT /api/staff/shifts/{shiftId}
 * Update a planned shift (staff assignment is immutable).
 */
export const PUT = proxyPut<unknown, UpdateShiftBody>(
  (p) => `/api/staff-shifts/${requirePathSegmentParam(p, "shiftId")}`,
);

/**
 * DELETE /api/staff/shifts/{shiftId}
 * Delete a planned shift.
 */
export const DELETE = proxyDelete(
  (p) => `/api/staff-shifts/${requirePathSegmentParam(p, "shiftId")}`,
);
