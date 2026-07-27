import { proxyPut } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

interface SplitSeriesBody {
  effective_date: string;
  start_time: string;
  end_time: string;
  break_minutes: number;
  shift_type_id: number | null;
  /** Omitted fields inherit the predecessor's value. valid_until is
   *  presence-aware: absent keeps the stored end, null runs to period end. */
  weekdays?: number[];
  week_pattern?: number;
  valid_until?: string | null;
  notes?: string;
}

/**
 * PUT /api/staff/shifts/series/{seriesId}/split
 * "Ab jetzt dauerhaft" and the series editor: caps the series at the effective
 * date and creates a successor carrying the edited rule (times, weekdays,
 * rhythm, validity); deviations move to the successor.
 */
export const PUT = proxyPut<unknown, SplitSeriesBody>(
  (p) =>
    `/api/staff-shifts/series/${requirePathSegmentParam(p, "seriesId")}/split`,
);
