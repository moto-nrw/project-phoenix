import { apiDelete } from "~/lib/api-helpers.server";
import { proxyGet } from "~/lib/route-proxy.server";
import {
  buildQueryString,
  requirePathSegmentParam,
} from "~/lib/route-wrapper-utils.server";
import { createDeleteHandler } from "~/lib/route-wrapper.server";

/**
 * GET /api/staff/shifts/series/{seriesId}
 * The stored rule behind a shift (weekdays, rhythm, validity) — what the
 * series editor loads before writing the change back through /split.
 */
export const GET = proxyGet(
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
