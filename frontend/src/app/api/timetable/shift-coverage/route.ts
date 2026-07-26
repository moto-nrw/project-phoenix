// Batched, read-only Dienstplan coverage probe. The backend requires both
// schedules:read and time_tracking:manage because exact shift gaps are
// cross-staff time-tracking data.
import { proxyPost } from "~/lib/route-proxy.server";

interface ShiftCoverageBody {
  dates: string[];
  start_time: string;
  end_time: string;
  staff_ids: number[];
  exclude_instance_id?: number;
  concrete_instance_date?: string;
  replan_activity_group_id?: number;
  calendar_period_id?: number;
  week_pattern?: number;
}

export const POST = proxyPost<unknown, ShiftCoverageBody>(
  "/api/timetable/shift-coverage",
);
