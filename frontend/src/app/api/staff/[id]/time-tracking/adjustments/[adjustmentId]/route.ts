import { proxyDelete } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

/**
 * DELETE /api/staff/[id]/time-tracking/adjustments/[adjustmentId]
 * Removes a Stundenkonto transaction (#1420) — admin typo-fix path, gated
 * server-side on time_tracking:manage.
 */
export const DELETE = proxyDelete((p) => {
  const adjustmentId = String(p.adjustmentId);
  if (!/^\d+$/.test(adjustmentId)) {
    throw new Error("Invalid adjustment id");
  }
  return `/api/staff/${requirePathSegmentParam(p)}/time-tracking/adjustments/${adjustmentId}`;
});
