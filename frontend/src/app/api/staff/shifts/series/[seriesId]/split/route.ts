import { proxyPut } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

interface SplitSeriesBody {
  effective_date: string;
  start_time: string;
  end_time: string;
  break_minutes: number;
  shift_type_id: number | null;
  /** Omitted fields (weekdays, week_pattern, notes) inherit the predecessor. */
  weekdays?: number[];
  week_pattern?: number;
  notes?: string;
}

/**
 * PUT /api/staff/shifts/series/{seriesId}/split
 * "Ab jetzt dauerhaft": caps the series at the effective date and creates a
 * successor with the edited fields; deviations move to the successor.
 */
export const PUT = proxyPut<unknown, SplitSeriesBody>(
  (p) =>
    `/api/staff-shifts/series/${requirePathSegmentParam(p, "seriesId")}/split`,
);
