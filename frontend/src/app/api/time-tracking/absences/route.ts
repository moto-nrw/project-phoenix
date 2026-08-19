import { proxyGet, proxyPost } from "~/lib/route-proxy.server";

/**
 * GET /api/time-tracking/absences?from=YYYY-MM-DD&to=YYYY-MM-DD
 * Get staff absences for date range
 */
export const GET = proxyGet("/api/time-tracking/absences");

interface CreateAbsenceBody {
  absence_type: string;
  /** School-defined Abwesenheitsart (#2403). */
  absence_type_id?: number | null;
  date_start: string;
  date_end: string;
  half_day?: boolean;
  note?: string;
}

/**
 * POST /api/time-tracking/absences
 * Create a new absence
 */
export const POST = proxyPost<unknown, CreateAbsenceBody>(
  "/api/time-tracking/absences",
);
