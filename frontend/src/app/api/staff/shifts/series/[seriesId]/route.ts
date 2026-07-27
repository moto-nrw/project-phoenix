import { apiDelete } from "~/lib/api-helpers.server";
import { proxyGet, proxyPut } from "~/lib/route-proxy.server";
import {
  buildQueryString,
  requirePathSegmentParam,
} from "~/lib/route-wrapper-utils.server";
import { createDeleteHandler } from "~/lib/route-wrapper.server";

/** Whole-series edit (#2028). Omitted optional fields keep the stored value;
 *  valid_until is presence-aware (null = run until the period ends). */
interface UpdateSeriesBody {
  weekdays?: number[];
  start_time: string;
  end_time: string;
  break_minutes: number;
  shift_type_id: number | null;
  week_pattern?: number;
  valid_until?: string | null;
}

/**
 * GET /api/staff/shifts/series/{seriesId}
 * The stored rule behind a shift (weekdays, rhythm, validity) — what the
 * planner edits when changing all occurrences of a series.
 */
export const GET = proxyGet(
  (p) => `/api/staff-shifts/series/${requirePathSegmentParam(p, "seriesId")}`,
);

/**
 * PUT /api/staff/shifts/series/{seriesId}
 * "Alle Termine der Serie": rewrites the rule and rebuilds every regenerable
 * future occurrence. Past days, cancellations, and deleted occurrences stay.
 */
export const PUT = proxyPut<unknown, UpdateSeriesBody>(
  (p) => `/api/staff-shifts/series/${requirePathSegmentParam(p, "seriesId")}`,
);

/**
 * DELETE /api/staff/shifts/series/{seriesId}?from=YYYY-MM-DD
 * End a shift series from the given date on; detached and past rows stay.
 * Hand-rolled instead of proxyDelete because the backend needs the `from`
 * query parameter, which proxyDelete does not forward.
 */
export const DELETE = createDeleteHandler(async (request, token, params) => {
  const seriesId = requirePathSegmentParam(params, "seriesId");
  await apiDelete(
    `/api/staff-shifts/series/${seriesId}${buildQueryString(request)}`,
    token,
  );
  return null;
});
