import { proxyGet, proxyPost } from "~/lib/route-proxy.server";

/**
 * GET /api/staff/shifts?from=YYYY-MM-DD&to=YYYY-MM-DD
 * All planned staff shifts in the date range (admin Dienstplan week view).
 */
export const GET = proxyGet("/api/staff-shifts");

interface CreateShiftBody {
  staff_id: number;
  date: string;
  start_time: string;
  end_time: string;
  break_minutes: number;
  cancelled?: boolean;
  change_reason?: string;
  origin_shift_id?: number;
}

/**
 * POST /api/staff/shifts
 * Create a planned shift.
 */
export const POST = proxyPost<unknown, CreateShiftBody>("/api/staff-shifts");
