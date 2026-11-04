import { proxyPut } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

/**
 * PUT /api/week-protocols/[protocolId]/notes
 * Saves the notes of a draft for the days and the week (#4769); a protocol
 * that is not a draft answers 409.
 */
export const PUT = proxyPut(
  (p) =>
    `/api/week-protocols/${requirePathSegmentParam(p, "protocolId")}/notes`,
);
