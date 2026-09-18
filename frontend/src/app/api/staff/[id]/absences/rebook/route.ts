import { proxyPost } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

/**
 * POST /api/staff/[id]/absences/rebook
 * Changes the type of stored absences, e.g. a past Freizeitausgleich into a
 * Krank-Urlaubstag (#3258). With `dry_run` the backend only describes the
 * effects. Gated server-side on time_tracking:manage.
 */
export const POST = proxyPost(
  (p) => `/api/staff/${requirePathSegmentParam(p)}/absences/rebook`,
);
