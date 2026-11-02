import { proxyPost } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

/**
 * POST /api/week-protocols/[protocolId]/close
 * Closes a draft (#4769). Afterwards the protocol no longer changes and its
 * PDF can be downloaded.
 */
export const POST = proxyPost(
  (p) =>
    `/api/week-protocols/${requirePathSegmentParam(p, "protocolId")}/close`,
);
