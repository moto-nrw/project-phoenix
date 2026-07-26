import { proxyPost } from "~/lib/route-proxy.server";

interface CreateSeriesBody {
  staff_id: number;
  weekdays: number[];
  start_time: string;
  end_time: string;
  break_minutes: number;
  shift_type_id: number | null;
  calendar_period_id: number;
  week_pattern: number;
  valid_from: string;
  valid_until: string | null;
}

/**
 * POST /api/staff/shifts/series
 * Create a recurring shift series (#1889); materializes concrete shifts
 * over the calendar period starting tomorrow.
 */
export const POST = proxyPost<unknown, CreateSeriesBody>(
  "/api/staff-shifts/series",
);
