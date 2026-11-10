import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyPost } from "~/lib/traeger/route-wrapper.server";

// Einen gespeicherten Jahresbescheid senden, zuerst oder erneut (#6347).
export const POST = proxyPost(
  (params) =>
    `/traeger/care-notice-runs/${requirePathSegmentParam(params)}/notices/${requirePathSegmentParam(params, "contractId")}/mail`,
);
