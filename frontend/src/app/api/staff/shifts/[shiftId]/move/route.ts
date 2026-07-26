import { proxyPut } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

interface MoveShiftBody {
  source_staff_id: number;
  target_staff_id: number;
  date: string;
  start_time: string;
  end_time: string;
  break_minutes: number;
  shift_type_id: number | null;
}

/** PUT /api/staff/shifts/{shiftId}/move — atomically move one shift. */
export const PUT = proxyPut<unknown, MoveShiftBody>(
  (p) => `/api/staff-shifts/${requirePathSegmentParam(p, "shiftId")}/move`,
);
