import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyPost } from "~/lib/traeger/route-wrapper.server";

// Ein SEPA-Mandat widerrufen (#4695).
export const POST = proxyPost(
  (params) =>
    `/traeger/sepa-mandates/${requirePathSegmentParam(params)}/revoke`,
);
