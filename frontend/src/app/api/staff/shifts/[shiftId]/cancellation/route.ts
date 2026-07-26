import { proxyPut } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

interface ReplacementBody {
  staff_id: number;
  start_time: string;
  end_time: string;
  break_minutes: number;
  shift_type_id: number | null;
}

interface CancellationBody {
  cancelled: boolean;
  change_reason: string;
  replacements: ReplacementBody[];
}

/**
 * PUT /api/staff/shifts/{shiftId}/cancellation
 * Atomically cancel/reactivate a shift and rebuild its replacement set (#1841).
 */
export const PUT = proxyPut<unknown, CancellationBody>(
  (p) =>
    `/api/staff-shifts/${requirePathSegmentParam(p, "shiftId")}/cancellation`,
);
