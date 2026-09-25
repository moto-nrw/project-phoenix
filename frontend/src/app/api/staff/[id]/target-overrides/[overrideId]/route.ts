import { proxyDelete } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

/**
 * DELETE /api/staff/[id]/target-overrides/[overrideId]
 * Removes a Sonderarbeitszeit (#3259); closed months are refused server-side.
 */
export const DELETE = proxyDelete((p) => {
  const overrideId = String(p.overrideId);
  if (!/^\d+$/.test(overrideId)) {
    throw new Error("Invalid override id");
  }
  return `/api/staff/${requirePathSegmentParam(p)}/target-overrides/${overrideId}`;
});
