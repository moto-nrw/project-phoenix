import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyGet } from "~/lib/traeger/route-wrapper.server";

// Eine Familie mit allen Dokumenten (#4691).
export const GET = proxyGet(
  (params) => `/traeger/billing-families/${requirePathSegmentParam(params)}`,
);
